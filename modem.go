package modemmanager

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
)

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
