package modemmanager

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func Test_valueParserErrors(t *testing.T) {
	tests := []struct {
		name string
		v    dbus.Variant
		fn   func(vp *valueParser)
	}{
		{
			name: "bool",
			v:    dbus.MakeVariant("foo"),
			fn: func(vp *valueParser) {
				_ = vp.Bool()
			},
		},
		{
			name: "float64",
			v:    dbus.MakeVariant("foo"),
			fn: func(vp *valueParser) {
				_ = vp.Float64()
			},
		},
		{
			name: "int",
			v:    dbus.MakeVariant(1.0),
			fn: func(vp *valueParser) {
				_ = vp.Int()
			},
		},
		{
			name: "IP type",
			v:    dbus.MakeVariant(1.0),
			fn: func(vp *valueParser) {
				_ = vp.IP()
			},
		},
		{
			name: "IP invalid",
			v:    dbus.MakeVariant("foo"),
			fn: func(vp *valueParser) {
				_ = vp.IP()
			},
		},
		{
			name: "mask",
			v:    dbus.MakeVariant(1.0),
			fn: func(vp *valueParser) {
				_ = vp.Mask(128)
			},
		},
		{
			name: "string",
			v:    dbus.MakeVariant(1),
			fn: func(vp *valueParser) {
				_ = vp.String()
			},
		},
		{
			name: "uint64",
			v:    dbus.MakeVariant("foo"),
			fn: func(vp *valueParser) {
				_ = vp.Uint64()
			},
		},
		{
			name: "object paths",
			v:    dbus.MakeVariant(1),
			fn: func(vp *valueParser) {
				_ = vp.ObjectPaths()
			},
		},
		{
			name: "ports type",
			v:    dbus.MakeVariant(1),
			fn: func(vp *valueParser) {
				_ = vp.Ports()
			},
		},
		{
			name: "ports slice",
			v: dbus.MakeVariant([][]interface{}{{
				"foo",
			}}),
			fn: func(vp *valueParser) {
				_ = vp.Ports()
			},
		},
		{
			name: "ports name",
			v: dbus.MakeVariant([][]interface{}{
				{
					true,
					1,
				},
			}),
			fn: func(vp *valueParser) {
				_ = vp.Ports()
			},
		},
		{
			name: "ports type",
			v: dbus.MakeVariant([][]interface{}{
				{
					"foo",
					true,
				},
			}),
			fn: func(vp *valueParser) {
				_ = vp.Ports()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Invoke the function and assume that any operation performed will
			// return an error.
			vp := newValueParser(tt.v)
			tt.fn(vp)
			err := vp.Err()
			if err == nil {
				t.Fatal("expected non-nil vp.Err() error, but none occurred")
			}

			t.Logf("err: %v", err)
		})
	}
}
