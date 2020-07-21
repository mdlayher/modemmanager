package modemmanager

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/google/go-cmp/cmp"
)

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
