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
type Modem struct {
	Index  int
	Device string

	c *Client
}

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

// GetNetworkTime fetches the current time from a Modem's network. If permission
// is denied by D-Bus, an error compatible with 'errors.Is(err,
// os.ErrPermission)' is returned.
func (m *Modem) GetNetworkTime(ctx context.Context) (time.Time, error) {
	v, err := m.c.call(
		ctx,
		interfacePath("Modem", "Time", "GetNetworkTime"),
		objectPath("Modem", strconv.Itoa(m.Index)),
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
	return time.Parse(time.RFC3339, str)
}

// parse parses a properties map into the Modem's fields.
func (m *Modem) parse(ps map[string]dbus.Variant) error {
	for k, v := range ps {
		// Parse every dbus.Variant as a well-typed value, or return an error
		// with vp.Err if the types don't match as expected.
		vp := newValueParser(v)
		switch k {
		case "Device":
			m.Device = vp.String()
		}

		if err := vp.Err(); err != nil {
			return fmt.Errorf("error parsing %q: %v", k, err)
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

// A callFunc is a function which calls a D-Bus method on an object.
type callFunc func(ctx context.Context, method string, op dbus.ObjectPath, args ...interface{}) (dbus.Variant, error)

// A getFunc is a function which fetches a D-Bus property from an object.
type getFunc func(ctx context.Context, op dbus.ObjectPath, iface, prop string) (dbus.Variant, error)

// A getAllFunc is a function which fetches all of an object's D-Bus properties.
type getAllFunc func(ctx context.Context, op dbus.ObjectPath, iface string) (map[string]dbus.Variant, error)

// makeCall produces a callFunc which call's a D-Bus method on an object.
func makeCall(c *dbus.Conn) callFunc {
	return func(ctx context.Context, method string, op dbus.ObjectPath, args ...interface{}) (dbus.Variant, error) {
		var out dbus.Variant
		if err := c.Object(service, op).CallWithContext(ctx, method, 0, args...).Store(&out); err != nil {
			return dbus.Variant{}, fmt.Errorf("failed to call %q: %w", method, err)
		}

		return out, nil
	}
}

// makeGet produces a getFunc which can fetch an object's property from a D-Bus
// interface.
func makeGet(c *dbus.Conn) getFunc {
	// Adapt a getFunc using the more generic callFunc.
	call := makeCall(c)
	return func(ctx context.Context, op dbus.ObjectPath, iface, prop string) (dbus.Variant, error) {
		out, err := call(ctx, methodGet, op, iface, prop)
		if err != nil {
			return dbus.Variant{}, fmt.Errorf("failed to get property %q for %q: %w",
				prop, iface, err)
		}

		return out, nil
	}
}

// makeGetAll produces a getAllFunc which fetches all of an object's properties
// from a D-Bus interface.
func makeGetAll(c *dbus.Conn) getAllFunc {
	return func(ctx context.Context, op dbus.ObjectPath, iface string) (map[string]dbus.Variant, error) {
		var out map[string]dbus.Variant
		if err := c.Object(service, op).CallWithContext(ctx, methodGetAll, 0, iface).Store(&out); err != nil {
			return nil, fmt.Errorf("failed to get all properties for %q: %w",
				iface, err)
		}

		return out, nil
	}
}

func panicf(format string, a ...interface{}) {
	panic(fmt.Sprintf(format, a...))
}
