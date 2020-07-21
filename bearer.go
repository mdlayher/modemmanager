package modemmanager

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
)

// A Bearer handles the cellular connection state of a Modem.
type Bearer struct {
	Index     int
	Connected bool
	Interface string
	IPTimeout time.Duration
	Suspended bool

	c *Client
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

// parse parses a properties map into the Bearer's fields.
func (b *Bearer) parse(ps map[string]dbus.Variant) error {
	for k, v := range ps {
		// Parse every dbus.Variant as a well-typed value, or return an error
		// with vp.Err if the types don't match as expected.
		vp := newValueParser(v)
		switch k {
		case "Connected":
			b.Connected = vp.Bool()
		case "Interface":
			b.Interface = vp.String()
		case "IpTimeout":
			b.IPTimeout = time.Duration(vp.Int()) * time.Second
		case "Suspended":
			b.Suspended = vp.Bool()
		}

		if err := vp.Err(); err != nil {
			return fmt.Errorf("error parsing %q: %v", k, err)
		}
	}

	return nil
}
