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
				// AWG 2.0 markers: S3/S4 plus H as ranges. protocol_version
				// is derived from these, so the fixture has to carry them.
				S1: 15, S2: 20, S3: 10, S4: 8,
				H1: "100-200", H2: "300-400", H3: "500-600", H4: "700-800",
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
				Config string  `json:"config"`
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
		S1:        15,
		S2:        20,
		S3:        10,
		S4:        8,
		H1:        "100-200",
		H2:        "300-400",
		H3:        "500-600",
		H4:        "700-800",
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

// decodeVPNURL unwraps a vpn:// payload back into the awg{} object.
func decodeVPNURL(t *testing.T, url string) map[string]any {
	t.Helper()
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
			AWG map[string]any `json:"awg"`
		} `json:"containers"`
	}
	if err := json.Unmarshal(body, &outer); err != nil {
		t.Fatalf("decode outer JSON: %v", err)
	}
	if len(outer.Containers) == 0 {
		t.Fatal("payload has no containers")
	}
	return outer.Containers[0].AWG
}

func syntheticManager(profile *Profile) *Manager {
	return &Manager{
		cfg: config.Config{
			WGHost:       "vpn.test.invalid",
			DNS:          "1.1.1.1",
			AllowedIPs:   "0.0.0.0/0",
			PersistentKA: 25,
		},
		profiles: map[string]*profileState{profile.ID: {profile: profile}},
		clients: map[string]*Client{
			"synthetic-client": {
				ID:         "synthetic-client",
				ProfileID:  profile.ID,
				Address:    "10.77.0.2",
				PrivateKey: "synthetic-client-private-key",
			},
		},
	}
}

func Test_AmneziaVPNURLWith_exports_AWG31_keys(t *testing.T) {
	// Given an AWG 3.1 profile
	profile := &Profile{
		ID:        "synthetic-profile",
		Name:      "Synthetic Profile",
		Port:      51820,
		PublicKey: "synthetic-server-public-key",
		S1:        100, S2: 120, S3: 30, S4: 12,
		H1: "1", H2: "2", H3: "3", H4: "4",
		I1:                     defaultI1CPS,
		HeaderProtectionKey:    "OjW5s9DDbnR/oPuMvHwOoHFHNXBhLUXcC0Wj4bDCOWQ=",
		ContentPaddingAddition: "10-100",
		RekeyAfterTime:         "100-120",
		RekeyTimeout:           "3-7",
		RejectAfterTime:        "150-180",
		KeepaliveTimeout:       "5-15",
		MaxHandshakeAttempts:   "15-20",
		RandomTrailers:         true,
		DisableCookies:         true,
		PersistentKeepalive:    "25-35",
	}

	// When
	url, err := syntheticManager(profile).AmneziaVPNURLWith("synthetic-client", "")
	if err != nil {
		t.Fatalf("generate AmneziaVPN URL: %v", err)
	}
	awgObj := decodeVPNURL(t, url)

	// Then: the 3.x keys ride at the awg{} top level under their conf names,
	// which is what AmneziaVPN 5.x reads.
	want := map[string]string{
		"HeaderProtectionKey":    "OjW5s9DDbnR/oPuMvHwOoHFHNXBhLUXcC0Wj4bDCOWQ=",
		"ContentPaddingAddition": "10-100",
		"RekeyAfterTime":         "100-120",
		"RekeyTimeout":           "3-7",
		"RejectAfterTime":        "150-180",
		"KeepaliveTimeout":       "5-15",
		"MaxHandshakeAttempts":   "15-20",
		"RandomTrailers":         "on",
		"DisableCookies":         "on",
	}
	for k, v := range want {
		if got, _ := awgObj[k].(string); got != v {
			t.Fatalf("awg[%q] = %v, want %q", k, awgObj[k], v)
		}
	}
	if got := awgObj["protocol_version"]; got != GenAWG31 {
		t.Fatalf("protocol_version = %v, want %q", got, GenAWG31)
	}

	// And again inside last_config, alongside the rendered conf.
	var last map[string]any
	if err := json.Unmarshal([]byte(awgObj["last_config"].(string)), &last); err != nil {
		t.Fatalf("decode last_config: %v", err)
	}
	for k, v := range want {
		if got, _ := last[k].(string); got != v {
			t.Fatalf("last_config[%q] = %v, want %q", k, last[k], v)
		}
	}
	if got, _ := last["persistent_keep_alive"].(string); got != "25-35" {
		t.Fatalf("persistent_keep_alive = %v, want the profile range", last["persistent_keep_alive"])
	}
	conf, _ := last["config"].(string)
	for _, line := range []string{
		"HeaderProtectionKey = OjW5s9DDbnR/oPuMvHwOoHFHNXBhLUXcC0Wj4bDCOWQ=",
		"RandomTrailers = on",
		"ContentPaddingAddition = 10-100",
		"PersistentKeepalive = 25-35",
	} {
		if !strings.Contains(conf, line) {
			t.Fatalf("nested conf missing %q:\n%s", line, conf)
		}
	}
}

// Test_AmneziaVPNURLWith_pre31_exports_stay_clean guards the other direction:
// an AWG 1.0/2.0 export must not sprout 3.x keys, since their mere presence is
// what makes the official client classify a config as v3.
func Test_AmneziaVPNURLWith_pre31_exports_stay_clean(t *testing.T) {
	profile := &Profile{
		ID:        "synthetic-profile",
		Name:      "Synthetic Profile",
		Port:      51820,
		PublicKey: "synthetic-server-public-key",
		S1:        15, S2: 20, S3: 10, S4: 8,
		H1: "100-200", H2: "300-400", H3: "500-600", H4: "700-800",
	}
	awgObj := decodeVPNURL(t, mustURL(t, syntheticManager(profile)))
	var last map[string]any
	if err := json.Unmarshal([]byte(awgObj["last_config"].(string)), &last); err != nil {
		t.Fatalf("decode last_config: %v", err)
	}
	for _, k := range []string{
		"HeaderProtectionKey", "ContentPaddingAddition", "RekeyAfterTime",
		"RekeyTimeout", "RejectAfterTime", "KeepaliveTimeout",
		"MaxHandshakeAttempts", "RandomTrailers", "DisableCookies",
	} {
		if _, ok := awgObj[k]; ok {
			t.Fatalf("AWG 2.0 export must not carry awg[%q]", k)
		}
		if _, ok := last[k]; ok {
			t.Fatalf("AWG 2.0 export must not carry last_config[%q]", k)
		}
	}
	if got := awgObj["protocol_version"]; got != "2" {
		t.Fatalf("protocol_version = %v, want \"2\"", got)
	}
}

func mustURL(t *testing.T, m *Manager) string {
	t.Helper()
	url, err := m.AmneziaVPNURLWith("synthetic-client", "")
	if err != nil {
		t.Fatalf("generate AmneziaVPN URL: %v", err)
	}
	return url
}
