package modemmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	// Fixed service, object prefix, etc. for communicating with ModemManager.
	service    = "org.freedesktop.ModemManager1"
	baseObject = dbus.ObjectPath("/org/freedesktop/ModemManager1")

	// Well-known method names.
	methodGet    = "org.freedesktop.DBus.Properties.Get"
	methodGetAll = "org.freedesktop.DBus.Properties.GetAll"

	// Well-known error names.
	unknownMethodError  = "org.freedesktop.DBus.Error.UnknownMethod"
	serviceUnknownError = "org.freedesktop.DBus.Error.ServiceUnknown"
)

// A Client allows control of ModemManager.
type Client struct {
	Version string

	// Functions which normally manipulate D-Bus but are also swappable for
	// tests.
	close  func() error
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

	c.Version = v.Value().(string)
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

// parse parses a properties map into the Modem's fields.
func (m *Modem) parse(ps map[string]dbus.Variant) error {
	for k, v := range ps {
		// TODO: copy the atmodem valueParser code.
		v := v.Value()
		switch k {
		case "Device":
			m.Device = v.(string)
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

	return fmt.Errorf("not found: %v: %w", derr, os.ErrNotExist)
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

// A getFunc is a function which fetches a D-Bus property from an object.
type getFunc func(ctx context.Context, op dbus.ObjectPath, iface, prop string) (dbus.Variant, error)

// A getAllFunc is a function which fetches all of an object's D-Bus properties.
type getAllFunc func(ctx context.Context, op dbus.ObjectPath, iface string) (map[string]dbus.Variant, error)

// makeGet produces a getFunc which can fetch an object's property from a D-Bus
// interface.
func makeGet(c *dbus.Conn) getFunc {
	return func(ctx context.Context, op dbus.ObjectPath, iface, prop string) (dbus.Variant, error) {
		obj := c.Object(service, op)

		var out dbus.Variant
		if err := obj.CallWithContext(ctx, methodGet, 0, iface, prop).Store(&out); err != nil {
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
		obj := c.Object(service, op)

		var out map[string]dbus.Variant
		if err := obj.CallWithContext(ctx, methodGetAll, 0, iface).Store(&out); err != nil {
			return nil, fmt.Errorf("failed to get all properties for %q: %w",
				iface, err)
		}

		return out, nil
	}
}

func panicf(format string, a ...interface{}) {
	panic(fmt.Sprintf(format, a...))
}