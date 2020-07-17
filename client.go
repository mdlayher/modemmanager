package modemmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	// Fixed service, object prefix, etc. for communicating with ModemManager.
	service    = "org.freedesktop.ModemManager1"
	baseObject = dbus.ObjectPath("/org/freedesktop/ModemManager1")

	// Well-known method names.
	methodGet    = "org.freedesktop.DBus.Properties.Get"
	methodGetAll = "org.freedesktop.DBus.Properties.GetAll"

	// Well-known error names which map to Go error types.
	//
	// os.ErrNotExist
	unknownMethodError  = "org.freedesktop.DBus.Error.UnknownMethod"
	serviceUnknownError = "org.freedesktop.DBus.Error.ServiceUnknown"
	// os.ErrPermission
	unauthorizedError = "org.freedesktop.ModemManager1.Error.Core.Unauthorized"
)

// A Client allows control of ModemManager.
type Client struct {
	Version string

	// Functions which normally manipulate D-Bus but are also swappable for
	// tests.
	close  func() error
	call   callFunc
	get    getFunc
	getAll getAllFunc
}

// Dial dials a D-Bus connection to ModemManager and returns a Client. If the
// ModemManager service does not exist, an error compatible with 'errors.Is(err,
// os.ErrNotExist)' is returned.
func Dial(ctx context.Context) (*Client, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	return initClient(ctx, &Client{
		// Wrap the *dbus.Conn completely to abstract away all of the low-level
		// D-Bus logic for ease of unit testing.
		close:  conn.Close,
		call:   makeCall(conn),
		get:    makeGet(conn),
		getAll: makeGetAll(conn),
	})
}

// initClient verifies a Client can speak with ModemManager.
func initClient(ctx context.Context, c *Client) (*Client, error) {
	// See if MM is available on the system bus by querying its version.
	v, err := c.get(ctx, baseObject, interfacePath(), "Version")
	if err != nil {
		// If not, D-Bus indicates service unknown when MM doesn't exist.
		return nil, toNotExist(err, serviceUnknownError)
	}

	vp := newValueParser(v)
	c.Version = vp.String()
	if err := vp.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse ModemManager version: %v", err)
	}

	return c, nil
}

// Close closes the underlying D-Bus connection.
func (c *Client) Close() error { return c.close() }

// A Modem is a device controlled by ModemManager.
//
// Calling methods on a modem requires elevated privileges. If permission is
// denied by D-Bus, an error compatible with 'errors.Is(err, os.ErrPermission)'
// is returned when methods are called.
type Modem struct {
	Index                        int
	CarrierConfiguration         string
	CarrierConfigurationRevision string
	Device                       string
	DeviceIdentifier             string
	EquipmentIdentifier          string
	HardwareRevision             string
	Manufacturer                 string
	Model                        string
	Plugin                       string
	Ports                        []Port
	PowerState                   PowerState
	PrimaryPort                  string
	Revision                     string
	State                        State

	c       *Client
	bearers []dbus.ObjectPath
}

//go:generate stringer -type=PortType,PowerState,State -output strings.go

// A PortType is the type of a modem port.
type PortType int

// Possible PortType values, taken from:
// https://www.freedesktop.org/software/ModemManager/api/latest/ModemManager-Flags-and-Enumerations.html#MMModemPortType.
const (
	PortTypeUnknown PortType = iota + 1
	PortTypeNet
	PortTypeAT
	PortTypeQCDM
	PortTypeGPS
	PortTypeQMI
	PortTypeMBIM
	PortTypeAudio
)

// A Port is a modem port.
type Port struct {
	Name string
	Type PortType
}

// A PowerState is the power state of a modem.
type PowerState int

// Possible PowerState values, taken from:
// https://www.freedesktop.org/software/ModemManager/api/latest/ModemManager-Flags-and-Enumerations.html#MMModemPowerState.
const (
	PowerStateUnknown PowerState = iota
	PowerStateOff
	PowerStateLow
	PowerStateOn
)

// A State is the state of a modem.
type State int

// Possible State values, taken from:
// https://www.freedesktop.org/software/ModemManager/api/latest/ModemManager-Flags-and-Enumerations.html#MMModemState.
const (
	StateFailed State = iota - 1
	StateUnknown
	StateInitializing
	StateLocked
	StateDisabled
	StateDisabling
	StateEnabling
	StateEnabled
	StateSearching
	StateRegistered
	StateDisconnecting
	StateConnecting
	StateConnected
)

// Modem fetches a Modem identified by an index. If the modem does not exist,
// an error compatible with 'errors.Is(err, os.ErrNotExist)' is returned.
func (c *Client) Modem(ctx context.Context, index int) (*Modem, error) {
	// Try to open the object for /Modem/N and fetch all the modem's properties
	// from the base Modem interface.
	ps, err := c.getAll(
		ctx,
		objectPath("Modem", strconv.Itoa(index)),
		interfacePath("Modem"),
	)
	if err != nil {
		// Unknown method indicates that the modem doesn't exist.
		return nil, toNotExist(err, unknownMethodError)
	}

	// Parse all of the properties into the Modem's exported fields.
	m := &Modem{
		Index: index,
		c:     c,
	}

	if err := m.parse(ps); err != nil {
		return nil, err
	}

	return m, nil
}

// ForEachModem iterates and invokes fn for each Modem fetched from
// ModemManager. Iteration halts when no more Modems exist or the input function
// returns an error.
func (c *Client) ForEachModem(ctx context.Context, fn func(ctx context.Context, m *Modem) error) error {
	for i := 0; ; i++ {
		m, err := c.Modem(ctx, i)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Halt iteration due to no more modems.
				return nil
			}

			return err
		}

		if err := fn(ctx, m); err != nil {
			return err
		}
	}
}

// GetNetworkTime fetches the current time from a Modem's network.
func (m *Modem) GetNetworkTime(ctx context.Context) (time.Time, error) {
	var v dbus.Variant
	err := m.c.call(
		ctx,
		interfacePath("Modem", "Time", "GetNetworkTime"),
		objectPath("Modem", strconv.Itoa(m.Index)),
		&v,
	)
	if err != nil {
		return time.Time{}, toPermission(err)
	}

	vp := newValueParser(v)
	str := vp.String()
	if err := vp.Err(); err != nil {
		return time.Time{}, err
	}

	// The time is actually ISO 8601 but it seems that RFC 3339 is close enough:
	// https://stackoverflow.com/questions/522251/whats-the-difference-between-iso-8601-and-rfc-3339-date-formats
	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return time.Time{}, err
	}

	// This feels like a hack and a misintepretation, but it seems that the time
	// string returned by modems contains a time zone offset but is actually
	// returned in UTC. We interpret the non-zone portions as a UTC time and
	// then convert that UTC time back to the time string's correct zone to
	// return the true time.
	y, mon, d := t.Date()
	hh, mm, ss := t.Clock()

	return time.Date(y, mon, d, hh, mm, ss, 0, time.UTC).In(t.Location()), nil
}

// SignalSetup sets the modem's extended signal quality refresh rate in seconds,
// enabling future calls to Signal to return updated signal strength data. Any
// fractional time values are rounded to the nearest second.
func (m *Modem) SignalSetup(ctx context.Context, rate time.Duration) error {
	err := m.c.call(
		ctx,
		interfacePath("Modem", "Signal", "Setup"),
		objectPath("Modem", strconv.Itoa(m.Index)),
		// No output, pass time in seconds as argument.
		nil,
		uint32(rate.Round(time.Second).Seconds()),
	)
	if err != nil {
		return toPermission(err)
	}

	return nil
}

// Signal contains cellular network extended signal quality information.
type Signal struct {
	Rate time.Duration
	LTE  struct {
		RSRP, RSRQ, RSSI, SNR float64
	}
}

// Signal returns cellular network extended signal quality information from the
// Modem. The refresh rate of the data can be controlled using SignalSetup.
func (m *Modem) Signal(ctx context.Context) (*Signal, error) {
	ps, err := m.c.getAll(
		ctx,
		objectPath("Modem", strconv.Itoa(m.Index)),
		interfacePath("Modem", "Signal"),
	)
	if err != nil {
		return nil, err
	}

	return parseSignal(ps)
}

// parse parses a properties map into the Modem's fields.
func (m *Modem) parse(ps map[string]dbus.Variant) error {
	for k, v := range ps {
		// Parse every dbus.Variant as a well-typed value, or return an error
		// with vp.Err if the types don't match as expected.
		vp := newValueParser(v)
		switch k {
		case "Bearers":
			m.bearers = vp.ObjectPaths()
		case "CarrierConfiguration":
			m.CarrierConfiguration = vp.String()
		case "CarrierConfigurationRevision":
			m.CarrierConfigurationRevision = vp.String()
		case "Device":
			m.Device = vp.String()
		case "DeviceIdentifier":
			m.DeviceIdentifier = vp.String()
		case "EquipmentIdentifier":
			m.EquipmentIdentifier = vp.String()
		case "HardwareRevision":
			m.HardwareRevision = vp.String()
		case "Manufacturer":
			m.Manufacturer = vp.String()
		case "Model":
			m.Model = vp.String()
		case "Plugin":
			m.Plugin = vp.String()
		case "Ports":
			m.Ports = vp.Ports()
		case "PowerState":
			m.PowerState = PowerState(vp.Int())
		case "PrimaryPort":
			m.PrimaryPort = vp.String()
		case "Revision":
			m.Revision = vp.String()
		case "State":
			m.State = State(vp.Int())
		}

		if err := vp.Err(); err != nil {
			return fmt.Errorf("error parsing %q: %v", k, err)
		}
	}

	return nil
}

// parseSignal parses a properties map into Signal data.
func parseSignal(ps map[string]dbus.Variant) (*Signal, error) {
	var s Signal
	for k, v := range ps {
		switch v := v.Value().(type) {
		case uint32:
			// Only Rate is expected for uint32.
			if k == "Rate" {
				s.Rate = time.Duration(v) * time.Second
			}
		case map[string]dbus.Variant:
			// Cellular network data maps.
			// TODO: parse other cellular network data.
			var err error
			switch k {
			case "Lte":
				l := &s.LTE
				err = parseLTESignal(v, &l.RSRP, &l.RSRQ, &l.RSSI, &l.SNR)
			}

			if err != nil {
				return nil, err
			}
		}
	}

	return &s, nil
}

// parseLTESignal parses a properties map into LTE data fields.
func parseLTESignal(ps map[string]dbus.Variant, rsrp, rsrq, rssi, snr *float64) error {
	for k, v := range ps {
		vp := newValueParser(v)
		switch k {
		case "rsrp":
			*rsrp = vp.Float64()
		case "rsrq":
			*rsrq = vp.Float64()
		case "rssi":
			*rssi = vp.Float64()
		case "snr":
			*snr = vp.Float64()
		}

		if err := vp.Err(); err != nil {
			return fmt.Errorf("error parsing LTE signal key %q: %v", k, err)
		}
	}

	return nil
}

// toNotExist converts a D-Bus error with the input name to a wrapped error
// containing os.ErrNotExist. If the error is not a dbus.Error or does not have
// a matching name, it returns the input error.
func toNotExist(err error, name string) error {
	var derr dbus.Error
	if !errors.As(err, &derr) || derr.Name != name {
		return err
	}

	// Also return the input error which may have wrapped the dbus.Error.
	return fmt.Errorf("not found: %v: %w", err, os.ErrNotExist)
}

// toPermission converts a D-Bus unauthorized error to a wrapped error
// containing os.ErrPermission. If the error is not a dbus.Error or does not
// have a matching name, it returns the input error.
func toPermission(err error) error {
	var derr dbus.Error
	if !errors.As(err, &derr) || derr.Name != unauthorizedError {
		return err
	}

	// Also return the input error which may have wrapped the dbus.Error.
	return fmt.Errorf("permission denied: %v: %w", err, os.ErrPermission)
}

// objectPath prepends its arguments with the base object path for ModemManager.
func objectPath(ss ...string) dbus.ObjectPath {
	p := dbus.ObjectPath(path.Join(
		// Prepend the base and join any further elements into one path.
		append([]string{string(baseObject)}, ss...)...,
	))

	// Since the paths in this program are effectively constant, they should
	// always be valid.
	if !p.IsValid() {
		panicf("modemmanager: bad D-Bus object path: %q", p)
	}

	return p
}

// interfacePath prepends its arguments with the base interface path for
// ModemManager.
func interfacePath(ss ...string) string {
	return strings.Join(append([]string{service}, ss...), ".")
}

// A callFunc is a function which calls a D-Bus method on an object and
// optionally stores its output in the pointer provided to out.
type callFunc func(ctx context.Context, method string, op dbus.ObjectPath, out interface{}, args ...interface{}) error

// A getFunc is a function which fetches a D-Bus property from an object.
type getFunc func(ctx context.Context, op dbus.ObjectPath, iface, prop string) (dbus.Variant, error)

// A getAllFunc is a function which fetches all of an object's D-Bus properties.
type getAllFunc func(ctx context.Context, op dbus.ObjectPath, iface string) (map[string]dbus.Variant, error)

// makeCall produces a callFunc which call's a D-Bus method on an object.
func makeCall(c *dbus.Conn) callFunc {
	return func(ctx context.Context, method string, op dbus.ObjectPath, out interface{}, args ...interface{}) error {
		call := c.Object(service, op).CallWithContext(ctx, method, 0, args...)
		if call.Err != nil {
			return fmt.Errorf("failed to call %q: %w", method, call.Err)
		}

		// Store the results of the call only when out is not nil.
		if out == nil {
			return nil
		}

		return call.Store(out)
	}
}

// makeGet produces a getFunc which can fetch an object's property from a D-Bus
// interface.
func makeGet(c *dbus.Conn) getFunc {
	// Adapt a getFunc using the more generic callFunc.
	call := makeCall(c)
	return func(ctx context.Context, op dbus.ObjectPath, iface, prop string) (dbus.Variant, error) {
		var out dbus.Variant
		if err := call(ctx, methodGet, op, &out, iface, prop); err != nil {
			return dbus.Variant{}, fmt.Errorf("failed to get property %q for %q: %w",
				prop, iface, err)
		}

		return out, nil
	}
}

// makeGetAll produces a getAllFunc which fetches all of an object's properties
// from a D-Bus interface.
func makeGetAll(c *dbus.Conn) getAllFunc {
	// Adapt a getAllFunc using the more generic callFunc.
	call := makeCall(c)
	return func(ctx context.Context, op dbus.ObjectPath, iface string) (map[string]dbus.Variant, error) {
		var out map[string]dbus.Variant
		if err := call(ctx, methodGetAll, op, &out, iface); err != nil {
			return nil, fmt.Errorf("failed to get all properties for %q: %w",
				iface, err)
		}

		return out, nil
	}
}

func panicf(format string, a ...interface{}) {
	panic(fmt.Sprintf(format, a...))
}
