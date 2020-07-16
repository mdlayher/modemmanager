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
			// Device must be a string, not an integer.
			return map[string]dbus.Variant{
				"Device": dbus.MakeVariant(1),
			}, nil
		},
	}

	_, err := c.Modem(context.Background(), 0)
	if err == nil {
		t.Fatal("expected an error, but none occurred")
	}

	t.Logf("err: %v", err)
}

func TestClientModemBadType(t *testing.T) {
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

			// Test data copied from mdlayher's modem with some tweaks.
			return map[string]dbus.Variant{
				"CarrierConfiguration":         dbus.MakeVariant(""),
				"CarrierConfigurationRevision": dbus.MakeVariant(""),
				"Device":                       dbus.MakeVariant("/sys/devices/pci0000:00/0000:00:13.0/usb1/1-1/1-1.3"),
				"DeviceIdentifier":             dbus.MakeVariant("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"),
				"EquipmentIdentifier":          dbus.MakeVariant("123456789012345"),
				"HardwareRevision":             dbus.MakeVariant("MC7455"),
				"Manufacturer":                 dbus.MakeVariant("Sierra Wireless, Incorporated"),
				"Model":                        dbus.MakeVariant("Sierra Wireless MC7455 Qualcomm® Snapdragon™ X7 LTE-A"),
				"Plugin":                       dbus.MakeVariant("Sierra"),
				"PrimaryPort":                  dbus.MakeVariant("cdc-wdm0"),
				"Revision":                     dbus.MakeVariant("SWI9X30C_02.33.03.00"),
			}, nil
		},
	}

	m, err := c.Modem(context.Background(), 0)
	if err != nil {
		t.Fatalf("failed to get modem 0: %v", err)
	}

	want := &Modem{
		Device:              "/sys/devices/pci0000:00/0000:00:13.0/usb1/1-1/1-1.3",
		DeviceIdentifier:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		EquipmentIdentifier: "123456789012345",
		HardwareRevision:    "MC7455",
		Manufacturer:        "Sierra Wireless, Incorporated",
		Model:               "Sierra Wireless MC7455 Qualcomm® Snapdragon™ X7 LTE-A",
		Plugin:              "Sierra",
		PrimaryPort:         "cdc-wdm0",
		Revision:            "SWI9X30C_02.33.03.00",
	}

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

func TestModemGetNetworkTimePermissionDenied(t *testing.T) {
	m := &Modem{
		c: &Client{call: func(_ context.Context, _ string, _ dbus.ObjectPath, _ interface{}, _ ...interface{}) error {
			// This is a privileged operation and D-Bus doesn't allow the caller
			// to perform it.
			return dbus.Error{Name: unauthorizedError}
		}},
	}

	_, err := m.GetNetworkTime(context.Background())
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected permission error, but got: %v", err)
	}

	t.Logf("err: %v", err)
}

func TestModemGetNetworkTimeOK(t *testing.T) {
	m := &Modem{
		c: &Client{call: func(_ context.Context, method string, op dbus.ObjectPath, out interface{}, args ...interface{}) error {
			if diff := cmp.Diff("org.freedesktop.ModemManager1.Modem.Time.GetNetworkTime", method); diff != "" {
				t.Fatalf("unexpected method (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(dbus.ObjectPath("/org/freedesktop/ModemManager1/Modem/0"), op); diff != "" {
				t.Fatalf("unexpected object path (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(0, len(args)); diff != "" {
				t.Fatalf("unexpected number of arguments (-want +got):\n%s", diff)
			}

			// Fixed time copied from mdlayher's modem.
			return dbus.Store([]interface{}{dbus.MakeVariant("2020-07-15T16:31:02-04:00")}, out)
		}},
	}

	now, err := m.GetNetworkTime(context.Background())
	if err != nil {
		t.Fatalf("failed to get network time: %v", err)
	}

	want := time.Date(2020, time.July, 15, 16, 31, 2, 0, time.FixedZone("EDT", -4*60*60))
	if diff := cmp.Diff(want, now); diff != "" {
		t.Fatalf("unexpected time (-want +got):\n%s", diff)
	}
}

func TestModemSignalSetup(t *testing.T) {
	m := &Modem{
		c: &Client{call: func(_ context.Context, method string, op dbus.ObjectPath, out interface{}, args ...interface{}) error {
			if diff := cmp.Diff("org.freedesktop.ModemManager1.Modem.Signal.Setup", method); diff != "" {
				t.Fatalf("unexpected method (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(dbus.ObjectPath("/org/freedesktop/ModemManager1/Modem/0"), op); diff != "" {
				t.Fatalf("unexpected object path (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(nil, out); diff != "" {
				t.Fatalf("unexpected out value (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff([]interface{}{uint32(10)}, args); diff != "" {
				t.Fatalf("unexpected arguments (-want +got):\n%s", diff)
			}

			// No return value.
			return nil
		}},
	}

	if err := m.SignalSetup(context.Background(), 10*time.Second); err != nil {
		t.Fatalf("failed to perform signal setup: %v", err)
	}
}

func TestModemSignal(t *testing.T) {
	m := &Modem{
		// Verify all of the expected inputs before returning canned properties.
		c: &Client{getAll: func(_ context.Context, op dbus.ObjectPath, dInterface string) (map[string]dbus.Variant, error) {
			if diff := cmp.Diff(dbus.ObjectPath("/org/freedesktop/ModemManager1/Modem/0"), op); diff != "" {
				t.Fatalf("unexpected object path (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff("org.freedesktop.ModemManager1.Modem.Signal", dInterface); diff != "" {
				t.Fatalf("unexpected interface (-want +got):\n%s", diff)
			}

			// Test data copied from mdlayher's modem with some tweaks.
			return map[string]dbus.Variant{
				"Rate": dbus.MakeVariant(uint32(10)),
				"Lte": dbus.MakeVariant(map[string]dbus.Variant{
					"rsrp": dbus.MakeVariant(float64(-117)),
					"rsrq": dbus.MakeVariant(float64(-14)),
					"rssi": dbus.MakeVariant(float64(-83)),
					"snr":  dbus.MakeVariant(float64(3)),
				}),
			}, nil
		}},
	}

	signal, err := m.Signal(context.Background())
	if err != nil {
		t.Fatalf("failed to get signal data: %v", err)
	}

	// TODO: reconsider use of anonymous structs if needed. They make tests more
	// ugly but keep the exported API more concise.

	want := &Signal{
		Rate: 10 * time.Second,
		LTE: struct {
			RSRP, RSRQ, RSSI, SNR float64
		}{
			RSRP: -117,
			RSRQ: -14,
			RSSI: -83,
			SNR:  3,
		},
	}

	if diff := cmp.Diff(want, signal); diff != "" {
		t.Fatalf("unexpected Signal (-want +got):\n%s", diff)
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
		t.Logf("- modem %d: %s", m.Index, m.Model)

		// Don't actually perform signal setup to avoid altering the modem's
		// state in a test, but fetch whatever data is available.
		signal, err := m.Signal(ctx)
		if err != nil {
			return err
		}

		t.Logf("  - signal: %+v", signal)

		// This is a privileged call, so don't fail the test if permission
		// is denied.
		switch now, err := m.GetNetworkTime(ctx); {
		case errors.Is(err, os.ErrPermission):
			t.Logf("  - time: (permission denied)")
		case err == nil:
			t.Logf("  - time: %s", now)
		default:
			return err
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to iterate modems: %v", err)
	}
}
