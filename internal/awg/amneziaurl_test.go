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
						LastConfig      string `json:"last_config"`
						ProtocolVersion string `json:"protocol_version"`
					} `json:"awg"`
				} `json:"containers"`
			}
			if err := json.Unmarshal(body, &outer); err != nil {
				t.Fatalf("decode outer JSON: %v", err)
			}
			if outer.DNS1 != tt.wantDNS1 || outer.DNS2 != tt.wantDNS2 {
				t.Fatalf("outer DNS = (%q, %q), want (%q, %q)", outer.DNS1, outer.DNS2, tt.wantDNS1, tt.wantDNS2)
			}
			if outer.Containers[0].AWG.ProtocolVersion != "2" {
				t.Fatalf("protocol_version = %q, want 2", outer.Containers[0].AWG.ProtocolVersion)
			}

			var last struct {
				Config string `json:"config"`
				I1     *string `json:"I1"`
				I2     *string `json:"I2"`
				I3     *string `json:"I3"`
				I4     *string `json:"I4"`
				I5     *string `json:"I5"`
			}
			if err := json.Unmarshal([]byte(outer.Containers[0].AWG.LastConfig), &last); err != nil {
				t.Fatalf("decode nested last_config: %v", err)
			}
			if !strings.Contains(last.Config, tt.wantDNSLine) {
				t.Fatalf("nested config does not contain %q:\n%s", tt.wantDNSLine, last.Config)
			}
			if strings.Contains(last.Config, "$PRIMARY_DNS") || strings.Contains(last.Config, "$SECONDARY_DNS") {
				t.Fatalf("nested config contains DNS placeholders:\n%s", last.Config)
			}
			// I1–I5 keys must always be present in last_config (may be empty).
			for name, p := range map[string]*string{"I1": last.I1, "I2": last.I2, "I3": last.I3, "I4": last.I4, "I5": last.I5} {
				if p == nil {
					t.Fatalf("last_config missing key %s", name)
				}
			}
		})
	}
}

func Test_AmneziaVPNURLWith_emits_I_chains_in_config_when_set(t *testing.T) {
	// Given a profile with AWG 2.0 I1–I5
	profile := &Profile{
		ID:        "synthetic-profile",
		Name:      "Synthetic Profile",
		Port:      51820,
		PublicKey: "synthetic-server-public-key",
		I1:        "<b 0xc100000001aabb><rc 12><t><r 20>",
		I2:        "<t><r 16><rc 8><rd 6>",
		I3:        "<t><r 80><rc 10><rd 8>",
		I4:        "<t><r 90><rc 4><rd 4>",
		I5:        "<t><r 70><rc 6><rd 6>",
	}
	m := &Manager{
		cfg: config.Config{
			WGHost:     "vpn.test.invalid",
			DNS:        "1.1.1.1",
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
	zr, err := zlib.NewReader(bytes.NewReader(payload[4:]))
	if err != nil {
		t.Fatalf("open zlib payload: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress payload: %v", err)
	}
	_ = zr.Close()

	var outer struct {
		Containers []struct {
			AWG struct {
				ProtocolVersion string `json:"protocol_version"`
				LastConfig      string `json:"last_config"`
			} `json:"awg"`
		} `json:"containers"`
	}
	if err := json.Unmarshal(body, &outer); err != nil {
		t.Fatalf("decode outer JSON: %v", err)
	}
	if outer.Containers[0].AWG.ProtocolVersion != "2" {
		t.Fatalf("protocol_version = %q, want 2", outer.Containers[0].AWG.ProtocolVersion)
	}

	var last struct {
		I1     string `json:"I1"`
		Config string `json:"config"`
	}
	if err := json.Unmarshal([]byte(outer.Containers[0].AWG.LastConfig), &last); err != nil {
		t.Fatalf("decode last_config: %v", err)
	}
	if last.I1 != profile.I1 {
		t.Fatalf("last_config.I1 = %q, want %q", last.I1, profile.I1)
	}
	if !strings.Contains(last.Config, "I1 = "+profile.I1) {
		t.Fatalf("nested conf missing I1:\n%s", last.Config)
	}
}
