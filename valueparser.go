package modemmanager

import (
	"errors"
	"fmt"
	"net"

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

// Bool parses the value as a bool.
func (vp *valueParser) Bool() bool {
	if vp.err != nil {
		return false
	}

	b, ok := vp.v.(bool)
	if !ok {
		vp.err = errors.New("value is not of type bool")
		return false
	}

	return b
}

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

// Int parses an int32 or uint32 value as a Go int.
func (vp *valueParser) Int() int {
	if vp.err != nil {
		return 0
	}

	switch v := vp.v.(type) {
	case uint32:
		return int(v)
	case int32:
		return int(v)
	default:
		vp.err = fmt.Errorf("value is not a valid integer: %T", v)
		return 0
	}
}

// IP parses a value as a net.IP
func (vp *valueParser) IP() net.IP {
	if vp.err != nil {
		return nil
	}

	s, ok := vp.v.(string)
	if !ok {
		vp.err = errors.New("value for IP is not of type string")
		return nil
	}

	ip := net.ParseIP(s)
	if ip == nil {
		vp.err = fmt.Errorf("invalid IP address: %q", s)
		return nil
	}

	return ip
}

// Mask parses a value as an IP address mask.
func (vp *valueParser) Mask(bits int) net.IPMask {
	// IP parses a value as a net.IP
	if vp.err != nil {
		return nil
	}

	i, ok := vp.v.(uint32)
	if !ok {
		vp.err = errors.New("value for IP mask is not of type uint32")
		return nil
	}

	return net.CIDRMask(int(i), bits)
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

// Uint64 parses the value as a uint64.
func (vp *valueParser) Uint64() uint64 {
	if vp.err != nil {
		return 0
	}

	u, ok := vp.v.(uint64)
	if !ok {
		vp.err = errors.New("value is not of type uint64")
		return 0
	}

	return u
}

// ObjectPaths parses the value as a slice of dbus.ObjectPaths.
func (vp *valueParser) ObjectPaths() []dbus.ObjectPath {
	if vp.err != nil {
		return nil
	}

	op, ok := vp.v.([]dbus.ObjectPath)
	if !ok {
		vp.err = errors.New("value is not an D-Bus object paths slice")
		return nil
	}

	return op
}

// Ports parses the value as a slice of Ports.
func (vp *valueParser) Ports() []Port {
	if vp.err != nil {
		return nil
	}

	// Ports data is packed in a slice of tuple slices with different data
	// types, so unfortunately we have to use empty interfaces and type
	// assertions:
	//
	// [["ttyUSB0", 1], ["wwan0", 2]], etc.

	ss, ok := vp.v.([][]interface{})
	if !ok {
		vp.err = errors.New("value is not a ports list")
		return nil
	}

	ps := make([]Port, 0, len(ss))
	for _, s := range ss {
		if len(s) != 2 {
			vp.err = errors.New("invalid ports list slice")
			return nil
		}

		name, ok := s[0].(string)
		if !ok {
			vp.err = errors.New("invalid port name string")
			return nil
		}

		typ, ok := s[1].(uint32)
		if !ok {
			vp.err = errors.New("invalid port type uint32")
			return nil
		}

		ps = append(ps, Port{
			Name: name,
			Type: PortType(typ),
		})
	}

	return ps
}

// Properties parses a value as a D-Bus properties map.
func (vp *valueParser) Properties() map[string]dbus.Variant {
	if vp.err != nil {
		return nil
	}

	ps, ok := vp.v.(map[string]dbus.Variant)
	if !ok {
		vp.err = errors.New("value is not a D-Bus properties map")
		return nil
	}

	return ps
}
