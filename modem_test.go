package modemmanager

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/google/go-cmp/cmp"
)

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

	want := time.Date(2020, time.July, 15, 12, 31, 2, 0, time.FixedZone("EDT", -4*60*60))
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
