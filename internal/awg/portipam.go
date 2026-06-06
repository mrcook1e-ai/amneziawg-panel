package awg

import (
	"errors"
	"fmt"
)

// PortIPAM hands out UDP ports inside a fixed range and maps each port back
// to a deterministic interface name (awg0 → rangeStart, awg1 → rangeStart+1, …).
// The mapping is deterministic so that a state.json + the same range
// reconstruct the same awgN names on every boot.
type PortIPAM struct {
	rangeStart int
	rangeEnd   int
}

func NewPortIPAM(start, end int) (*PortIPAM, error) {
	if start <= 0 || end < start {
		return nil, fmt.Errorf("invalid port range: %d..%d", start, end)
	}
	return &PortIPAM{rangeStart: start, rangeEnd: end}, nil
}

func (p *PortIPAM) Range() (int, int) { return p.rangeStart, p.rangeEnd }

func (p *PortIPAM) Next(used map[int]struct{}) (int, error) {
	for port := p.rangeStart; port <= p.rangeEnd; port++ {
		if _, taken := used[port]; !taken {
			return port, nil
		}
	}
	return 0, errors.New("port range exhausted")
}

func (p *PortIPAM) IfaceFor(port int) string {
	return fmt.Sprintf("awg%d", port-p.rangeStart)
}

func (p *PortIPAM) Valid(port int) bool {
	return port >= p.rangeStart && port <= p.rangeEnd
}
