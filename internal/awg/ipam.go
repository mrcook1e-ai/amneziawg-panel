package awg

import (
	"errors"
	"net/netip"
	"strings"
)

// IPAM allocates IPs inside a /24 derived from a "10.8.0.x" pattern.
// Server gets .1, clients start at .2.
type IPAM struct {
	base    netip.Addr // 10.8.0.0
	pattern string     // "10.8.0.x"
}

func NewIPAM(pattern string) (*IPAM, error) {
	if !strings.Contains(pattern, "x") {
		return nil, errors.New("subnet pattern must contain 'x'")
	}
	zero, err := netip.ParseAddr(strings.Replace(pattern, "x", "0", 1))
	if err != nil {
		return nil, err
	}
	return &IPAM{base: zero, pattern: pattern}, nil
}

func (a *IPAM) ServerIP() string { return strings.Replace(a.pattern, "x", "1", 1) }

func (a *IPAM) Next(used map[string]struct{}) (string, error) {
	for i := 2; i < 255; i++ {
		ip := strings.Replace(a.pattern, "x", itoa(i), 1)
		if _, taken := used[ip]; !taken {
			return ip, nil
		}
	}
	return "", errors.New("subnet exhausted")
}

func (a *IPAM) Valid(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return false
	}
	b := a.base.As4()
	x := addr.As4()
	return b[0] == x[0] && b[1] == x[1] && b[2] == x[2] && x[3] >= 2 && x[3] <= 254
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [4]byte
	n := 0
	for i > 0 {
		buf[n] = byte('0' + i%10)
		n++
		i /= 10
	}
	out := make([]byte, n)
	for k := 0; k < n; k++ {
		out[k] = buf[n-1-k]
	}
	return string(out)
}
