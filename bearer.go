package modemmanager

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path"
	"sort"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
)

// A Bearer handles the cellular connection state of a Modem.
type Bearer struct {
	Index                  int
	Connected              bool
	Interface              string
	IPTimeout              time.Duration
	IPv4Config, IPv6Config *IPConfig
	Suspended              bool

	c *Client
}

// A BearerIPMethod is the method a Bearer must use to obtain IP address
// configuration.
type BearerIPMethod int

// Possible BearerIPMethod values, taken from:
// https://www.freedesktop.org/software/ModemManager/api/latest/ModemManager-Flags-and-Enumerations.html#MMBearerIpMethod.
const (
	BearerIPMethodUnknown BearerIPMethod = iota
	BearerIPMethodPPP
	BearerIPMethodStatic
	BearerIPMethodDHCP
)

// An IPConfig is a Bearer's IPv4 or IPv6 configuration.
type IPConfig struct {
	Address *net.IPNet
	DNS     []net.IP
	Gateway net.IP
	Method  BearerIPMethod
	MTU     int
}

// Bearers returns all of the Bearers for a Modem.
func (m *Modem) Bearers(ctx context.Context) ([]*Bearer, error) {
	bs := make([]*Bearer, 0, len(m.bearers))
	for _, op := range m.bearers {
		// Fetch all of the properties from the Bearers associated with this
		// Modem.
		ps, err := m.c.getAll(
			ctx,
			op,
			interfacePath("Bearer"),
		)
		if err != nil {
			return nil, err
		}

		// Note the Bearer's index in the struct by fetching that index from
		// the last element of the D-Bus object path.
		idx, err := strconv.Atoi(path.Base(string(op)))
		if err != nil {
			return nil, err
		}

		// Parse all of the properties into the Bearer's exported fields.
		b := &Bearer{
			Index: idx,
			c:     m.c,
		}

		if err := b.parse(ps); err != nil {
			return nil, err
		}

		bs = append(bs, b)
	}

	return bs, nil
}

// Friendly names for IPv4/6 control flow booleans.
const (
	isIPv4 = false
	isIPv6 = true
)

// parse parses a properties map into the Bearer's fields.
func (b *Bearer) parse(ps map[string]dbus.Variant) error {
	for k, v := range ps {
		vp := newValueParser(v)
		switch k {
		case "Connected":
			b.Connected = vp.Bool()
		case "Interface":
			b.Interface = vp.String()
		case "IpTimeout":
			b.IPTimeout = time.Duration(vp.Int()) * time.Second
		case "Ip4Config":
			c, err := parseIPConfig(vp.Properties(), isIPv4)
			if err != nil {
				return fmt.Errorf("error parsing IPv4 config: %v", err)
			}
			b.IPv4Config = c
		case "Ip6Config":
			c, err := parseIPConfig(vp.Properties(), isIPv6)
			if err != nil {
				return fmt.Errorf("error parsing IPv6 config: %v", err)
			}
			b.IPv6Config = c
		case "Suspended":
			b.Suspended = vp.Bool()
		}

		if err := vp.Err(); err != nil {
			return fmt.Errorf("error parsing %q: %v", k, err)
		}
	}

	return nil
}

// parseIPConfig parses IPv4 or IPv6 configuration from a properties map.
func parseIPConfig(ps map[string]dbus.Variant, ip6 bool) (*IPConfig, error) {
	var c IPConfig

	// The expected mask size.
	bits := 32
	if ip6 {
		bits = 128
	}

	for k, v := range ps {
		vp := newValueParser(v)
		switch k {
		case "address":
			if c.Address == nil {
				c.Address = &net.IPNet{}
			}

			c.Address.IP = vp.IP()
		case "dns1", "dns2", "dns3":
			c.DNS = append(c.DNS, vp.IP())
		case "gateway":
			c.Gateway = vp.IP()
		case "method":
			c.Method = BearerIPMethod(vp.Int())
		case "mtu":
			c.MTU = vp.Int()
		case "prefix":
			if c.Address == nil {
				c.Address = &net.IPNet{}
			}

			c.Address.Mask = vp.Mask(bits)
		}
	}

	// Sort DNS addresses for consistency.
	sort.SliceStable(c.DNS, func(i, j int) bool {
		return bytes.Compare(c.DNS[i], c.DNS[j]) == -1
	})

	return &c, nil
}
