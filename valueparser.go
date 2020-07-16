package modemmanager

import (
	"errors"

	"github.com/godbus/dbus/v5"
)

// A valueParser parses well-typed values from an empty interface value.
//
// After each parsing operation, the caller must invoke the Err method to
// determine if any input could not be parsed as the specified type.
type valueParser struct {
	v   interface{}
	err error
}

// newValueParser constructs a valueParser from a dbus.Variant value.
func newValueParser(v dbus.Variant) *valueParser {
	return &valueParser{v: v.Value()}
}

// Err returns the current parsing error, if there is one.
func (vp *valueParser) Err() error { return vp.err }

// Float64 parses the value as a float64.
func (vp *valueParser) Float64() float64 {
	if vp.err != nil {
		return 0
	}

	f, ok := vp.v.(float64)
	if !ok {
		vp.err = errors.New("value is not of type float64")
		return 0
	}

	return f
}

// String parses the value as a string.
func (vp *valueParser) String() string {
	if vp.err != nil {
		return ""
	}

	s, ok := vp.v.(string)
	if !ok {
		vp.err = errors.New("value is not of type string")
		return ""
	}

	return s
}
