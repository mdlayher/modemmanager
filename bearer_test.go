package modemmanager

import (
	"context"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestModemBearers(t *testing.T) {
	m := &Modem{
		// Verify all of the expected inputs before returning canned properties.
		c: &Client{getAll: func(_ context.Context, op dbus.ObjectPath, dInterface string) (map[string]dbus.Variant, error) {
			if !strings.HasPrefix(string(op), "/org/freedesktop/ModemManager1/Bearer/") {
				t.Fatalf("unexpected object path: %q", op)
			}

			if diff := cmp.Diff("org.freedesktop.ModemManager1.Bearer", dInterface); diff != "" {
				t.Fatalf("unexpected interface (-want +got):\n%s", diff)
			}

			// Only return full properties for the first bearer.
			if path.Base(string(op)) != "0" {
				return nil, nil
			}

			// Test data copied from mdlayher's modem with some tweaks.
			return map[string]dbus.Variant{
				"Connected": dbus.MakeVariant(true),
				"Interface": dbus.MakeVariant("wwan0"),
				"IpTimeout": dbus.MakeVariant(uint32(20)),
				"Suspended": dbus.MakeVariant(false),
			}, nil
		}},

		bearers: []dbus.ObjectPath{
			"/org/freedesktop/ModemManager1/Bearer/0",
			"/org/freedesktop/ModemManager1/Bearer/1",
		},
	}

	bearers, err := m.Bearers(context.Background())
	if err != nil {
		t.Fatalf("failed to get bearers: %v", err)
	}

	want := []*Bearer{
		{
			Index:     0,
			Connected: true,
			Interface: "wwan0",
			IPTimeout: 20 * time.Second,
			Suspended: false,
		},
		{
			Index:     1,
			Connected: false,
		},
	}

	if diff := cmp.Diff(want, bearers, cmpopts.IgnoreUnexported(Bearer{})); diff != "" {
		t.Fatalf("unexpected Bearers (-want +got):\n%s", diff)
	}
}
