package awg

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/mrcook1e/amneziawg-panel/internal/config"
)

func Test_AmneziaVPNURLWith_renders_configured_DNS_in_nested_config(t *testing.T) {
	tests := []struct {
		name        string
		dns         string
		wantDNS1    string
		wantDNS2    string
		wantDNSLine string
	}{
		{
			name:        "primary and secondary DNS",
			dns:         "203.0.113.53, 198.51.100.54",
			wantDNS1:    "203.0.113.53",
			wantDNS2:    "198.51.100.54",
			wantDNSLine: "DNS = 203.0.113.53, 198.51.100.54\n",
		},
		{
			name:        "primary DNS only",
			dns:         "192.0.2.53",
			wantDNS1:    "192.0.2.53",
			wantDNS2:    "",
			wantDNSLine: "DNS = 192.0.2.53\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			profile := &Profile{
				ID:        "synthetic-profile",
				Name:      "Synthetic Profile",
				Port:      51820,
				PublicKey: "synthetic-server-public-key",
			}
			m := &Manager{
				cfg: config.Config{
					WGHost:     "vpn.test.invalid",
					DNS:        tt.dns,
					AllowedIPs: "0.0.0.0/0",
				},
				profiles: map[string]*profileState{
					profile.ID: {profile: profile},
				},
				clients: map[string]*Client{
					"synthetic-client": {
						ID:         "synthetic-client",
						ProfileID:  profile.ID,
						Address:    "10.77.0.2",
						PrivateKey: "synthetic-client-private-key",
					},
				},
			}

			// When
			url, err := m.AmneziaVPNURLWith("synthetic-client", "")
			if err != nil {
				t.Fatalf("generate AmneziaVPN URL: %v", err)
			}

			// Then
			payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(url, "vpn://"))
			if err != nil {
				t.Fatalf("decode base64 payload: %v", err)
			}
			if len(payload) < 4 {
				t.Fatalf("payload has no uint32 length prefix: %d bytes", len(payload))
			}
			zr, err := zlib.NewReader(bytes.NewReader(payload[4:]))
			if err != nil {
				t.Fatalf("open zlib payload: %v", err)
			}
			body, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("decompress payload: %v", err)
			}
			if err := zr.Close(); err != nil {
				t.Fatalf("close zlib payload: %v", err)
			}
			if got, want := uint32(len(body)), binary.BigEndian.Uint32(payload[:4]); got != want {
				t.Fatalf("uncompressed length = %d, want %d", got, want)
			}

			var outer struct {
				DNS1       string `json:"dns1"`
				DNS2       string `json:"dns2"`
				Containers []struct {
					AWG struct {
						LastConfig string `json:"last_config"`
					} `json:"awg"`
				} `json:"containers"`
			}
			if err := json.Unmarshal(body, &outer); err != nil {
				t.Fatalf("decode outer JSON: %v", err)
			}
			if outer.DNS1 != tt.wantDNS1 || outer.DNS2 != tt.wantDNS2 {
				t.Fatalf("outer DNS = (%q, %q), want (%q, %q)", outer.DNS1, outer.DNS2, tt.wantDNS1, tt.wantDNS2)
			}

			lastConfig := outer.Containers[0].AWG.LastConfig
			if !strings.HasPrefix(lastConfig, "[Interface]\n") {
				t.Fatalf("last_config must be an INI configuration, got %q", lastConfig)
			}
			if !strings.Contains(lastConfig, tt.wantDNSLine) {
				t.Fatalf("nested config does not contain %q:\n%s", tt.wantDNSLine, lastConfig)
			}
			if strings.Contains(lastConfig, "$PRIMARY_DNS") || strings.Contains(lastConfig, "$SECONDARY_DNS") {
				t.Fatalf("nested config contains DNS placeholders:\n%s", lastConfig)
			}
		})
	}
}
