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
			name: "bad float64",
			v:    dbus.MakeVariant("foo"),
			fn: func(vp *valueParser) {
				_ = vp.Float64()
			},
		},
		{
			name: "bad string",
			v:    dbus.MakeVariant(1),
			fn: func(vp *valueParser) {
				_ = vp.String()
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
