package modemmanager

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/godbus/dbus/v5"
)

// Signal contains cellular network extended signal quality information.
type Signal struct {
	Rate time.Duration
	LTE  struct {
		RSRP, RSRQ, RSSI, SNR float64
	}
}

// Signal returns cellular network extended signal quality information from the
// Modem. The refresh rate of the data can be controlled using SignalSetup.
func (m *Modem) Signal(ctx context.Context) (*Signal, error) {
	ps, err := m.c.getAll(
		ctx,
		objectPath("Modem", strconv.Itoa(m.Index)),
		interfacePath("Modem", "Signal"),
	)
	if err != nil {
		return nil, err
	}

	return parseSignal(ps)
}

// parseSignal parses a properties map into Signal data.
func parseSignal(ps map[string]dbus.Variant) (*Signal, error) {
	var s Signal
	for k, v := range ps {
		switch v := v.Value().(type) {
		case uint32:
			// Only Rate is expected for uint32.
			if k == "Rate" {
				s.Rate = time.Duration(v) * time.Second
			}
		case map[string]dbus.Variant:
			// Cellular network data maps.
			// TODO: parse other cellular network data.
			var err error
			switch k {
			case "Lte":
				l := &s.LTE
				err = parseLTESignal(v, &l.RSRP, &l.RSRQ, &l.RSSI, &l.SNR)
			}

			if err != nil {
				return nil, err
			}
		}
	}

	return &s, nil
}

// parseLTESignal parses a properties map into LTE data fields.
func parseLTESignal(ps map[string]dbus.Variant, rsrp, rsrq, rssi, snr *float64) error {
	for k, v := range ps {
		vp := newValueParser(v)
		switch k {
		case "rsrp":
			*rsrp = vp.Float64()
		case "rsrq":
			*rsrq = vp.Float64()
		case "rssi":
			*rssi = vp.Float64()
		case "snr":
			*snr = vp.Float64()
		}

		if err := vp.Err(); err != nil {
			return fmt.Errorf("error parsing LTE signal key %q: %v", k, err)
		}
	}

	return nil
}
