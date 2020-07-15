package modemmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func Test_initClientOK(t *testing.T) {
	const version = "1.99.99"

	c, err := initClient(context.Background(), &Client{
		close: func() error { return nil },
		// Verify all of the expected inputs before returning a canned version
		// value.
		get: func(_ context.Context, op dbus.ObjectPath, dInterface, prop string) (dbus.Variant, error) {
			if diff := cmp.Diff(baseObject, op); diff != "" {
				t.Fatalf("unexpected object path (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(service, dInterface); diff != "" {
				t.Fatalf("unexpected interface (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff("Version", prop); diff != "" {
				t.Fatalf("unexpected property (-want +got):\n%s", diff)
			}

			return dbus.MakeVariant(version), nil
		},
	})
	if err != nil {
		t.Fatalf("failed to create Client: %v", err)
	}
	defer c.Close()

	if diff := cmp.Diff(version, c.Version); diff != "" {
		t.Fatalf("unexpected version (-want +got):\n%s", diff)
	}
}

func Test_initClientNotFound(t *testing.T) {
	_, err := initClient(context.Background(), &Client{
		close: func() error { return nil },
		get: func(_ context.Context, _ dbus.ObjectPath, _, _ string) (dbus.Variant, error) {
			// D-Bus returns "service unknown" when MM isn't available.
			return dbus.Variant{}, dbus.Error{Name: serviceUnknownError}
		},
	})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected is not exist error, but got: %v", err)
	}

	t.Logf("err: %v", err)
}

func Test_initClientError(t *testing.T) {
	_, err := initClient(context.Background(), &Client{
		close: func() error { return nil },
		get: func(_ context.Context, _ dbus.ObjectPath, _, _ string) (dbus.Variant, error) {
			// Some unhandled error.
			return dbus.Variant{}, dbus.Error{Name: unknownMethodError}
		},
	})
	if err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected error other than not exist, but got: %v", err)
	}

	t.Logf("err: %v", err)
}

func TestClientModemNotFound(t *testing.T) {
	c := &Client{
		getAll: func(_ context.Context, _ dbus.ObjectPath, _ string) (map[string]dbus.Variant, error) {
			// D-Bus returns "unknown method" when a modem doesn't exist at the
			// input index.
			return nil, dbus.Error{Name: unknownMethodError}
		},
	}

	_, err := c.Modem(context.Background(), 0)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected is not exist error, but got: %v", err)
	}

	t.Logf("err: %v", err)
}

func TestClientModemOK(t *testing.T) {
	c := &Client{
		// Verify all of the expected inputs before returning canned properties.
		getAll: func(_ context.Context, op dbus.ObjectPath, dInterface string) (map[string]dbus.Variant, error) {
			if diff := cmp.Diff(dbus.ObjectPath("/org/freedesktop/ModemManager1/Modem/0"), op); diff != "" {
				t.Fatalf("unexpected object path (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff("org.freedesktop.ModemManager1.Modem", dInterface); diff != "" {
				t.Fatalf("unexpected interface (-want +got):\n%s", diff)
			}

			// TODO: return more fields!
			return map[string]dbus.Variant{
				"Device": dbus.MakeVariant("test"),
			}, nil
		},
	}

	m, err := c.Modem(context.Background(), 0)
	if err != nil {
		t.Fatalf("failed to get modem 0: %v", err)
	}

	// TODO: parse more fields!
	want := &Modem{Device: "test"}
	if diff := cmp.Diff(want, m, cmpopts.IgnoreUnexported(Modem{})); diff != "" {
		t.Fatalf("unexpected Modem (-want +got):\n%s", diff)
	}
}

func TestClientForEachModemOK(t *testing.T) {
	var count int
	c := &Client{
		getAll: func(_ context.Context, _ dbus.ObjectPath, _ string) (map[string]dbus.Variant, error) {
			// Count the number of modems returned and eventually end iteration
			// by returning unknown method.
			defer func() { count++ }()
			if count > 2 {
				return nil, dbus.Error{Name: unknownMethodError}
			}

			return map[string]dbus.Variant{
				"Device": dbus.MakeVariant(fmt.Sprintf("test%d", count)),
			}, nil
		},
	}

	// Gather all of the possible modems for comparison.
	var modems []*Modem
	err := c.ForEachModem(context.Background(), func(_ context.Context, m *Modem) error {
		modems = append(modems, m)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to iterate modems: %v", err)
	}

	want := []*Modem{
		{
			Index:  0,
			Device: "test0",
		},
		{
			Index:  1,
			Device: "test1",
		},
		{
			Index:  2,
			Device: "test2",
		},
	}

	if diff := cmp.Diff(want, modems, cmpopts.IgnoreUnexported(Modem{})); diff != "" {
		t.Fatalf("unexpected modems (-want +got):\n%s", diff)
	}
}

func TestClientForEachModemError(t *testing.T) {
	c := &Client{
		getAll: func(_ context.Context, _ dbus.ObjectPath, _ string) (map[string]dbus.Variant, error) {
			// Always return a modem.
			return map[string]dbus.Variant{
				"Device": dbus.MakeVariant("test"),
			}, nil
		},
	}

	err := c.ForEachModem(context.Background(), func(_ context.Context, _ *Modem) error {
		// Suppose the caller invokes a privileged method here which returns an
		// error due to insufficient permissions.
		return os.ErrPermission
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected permission denied, but got: %v", err)
	}
}

func TestIntegrationClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check for the availability of MM.
	c, err := Dial(ctx)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skipf("skipping, ModemManager not found: %v", err)
		}

		t.Fatalf("failed to dial: %v", err)
	}
	defer c.Close()

	t.Logf("ModemManager: v%s", c.Version)

	// Iterate through each connected modem until a new modem does not exist.
	err = c.ForEachModem(ctx, func(ctx context.Context, m *Modem) error {
		t.Logf("- modem %d: %s", m.Index, m.Device)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to iterate modems: %v", err)
	}
}
