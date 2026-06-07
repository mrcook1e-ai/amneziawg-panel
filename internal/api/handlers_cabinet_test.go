package api

import (
	"strings"
	"testing"
)

func TestSanitizeAllowedIPs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   ,  , ", ""},
		{"single v4 cidr", "10.0.0.0/8", "10.0.0.0/8"},
		{"v4 default", "0.0.0.0/0", "0.0.0.0/0"},
		{"v6 default", "::/0", "::/0"},
		{"multiple, normalized", "10.0.0.0/8, 192.168.1.0/24", "10.0.0.0/8, 192.168.1.0/24"},
		{"dedup", "10.0.0.0/8, 10.0.0.0/8", "10.0.0.0/8"},
		// ParseCIDR normalises the host portion: "10.0.0.5/8" → "10.0.0.0/8".
		{"normalize host bits", "10.0.0.5/8", "10.0.0.0/8"},
		// Bare IP without mask is invalid for AllowedIPs — drop, don't auto-add /32.
		{"bare ip dropped", "8.8.8.8, 10.0.0.0/8", "10.0.0.0/8"},
		{"junk dropped", "; rm -rf /, 10.0.0.0/8", "10.0.0.0/8"},
		{"crlf in entry", "10.0.0.0/8\r\n, 192.168.0.0/16", "10.0.0.0/8, 192.168.0.0/16"},
		{"all invalid", "foo, bar, baz", ""},
		// Raw input >32 KiB is rejected wholesale (4000×10 = 40 KiB).
		{"too long", strings.Repeat("0.0.0.0/0,", 4000), ""},
		// Just under cap: 3000 dup entries fold to one after dedup.
		{"high count dedup", strings.Repeat("0.0.0.0/0,", 3000), "0.0.0.0/0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeAllowedIPs(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeAllowedIPs(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
