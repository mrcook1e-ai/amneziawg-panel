package awg

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// AmneziaVPNURL builds a `vpn://AAAN...` URL compatible with the official
// AmneziaVPN client (https://github.com/amnezia-vpn/amnezia-client).
//
// Wire format (reverse-engineered from config-decoder + awg-converter):
//
//	url  = "vpn://" + base64url_nopad(blob)
//	blob = uint32_be(len(json)) || zlib(json)
//
// The 4-byte prefix is Qt qCompress framing. qUncompress expects it before
// the zlib stream; we produce a standard zlib stream (compress/zlib), which
// is what Qt decompresses correctly.
//
// JSON structure mirrors the working awg-converter implementation:
//   - dns1 / dns2 at root level
//   - AWG params both at awg{} top level and inside last_config
//   - last_config.config uses the effective DNS configuration
//   - isThirdPartyConfig: true (skips server-side SSH/Docker on import)
func (m *Manager) AmneziaVPNURL(deviceID string) (string, error) {
	return m.AmneziaVPNURLWith(deviceID, "")
}

// AmneziaVPNURLWith builds the same payload as AmneziaVPNURL but with an
// explicit AllowedIPs override, used by the user-facing split-tunnel UI
// in the cabinet. Precedence (first non-empty wins):
//
//  1. allowedIPsOverride argument (request-time choice from cabinet)
//  2. client.AllowedIPsOverride  (admin-set per-client default)
//  3. m.cfg.AllowedIPs           (server default)
//
// The override is NOT persisted — different downloads of the same device
// can carry different routes. This matches the user expectation: pick
// services, scan QR / download .vpn, re-import. Admin-set override remains
// the floor for users who never touch the cabinet's split-tunnel sheet.
func (m *Manager) AmneziaVPNURLWith(deviceID, allowedIPsOverride string) (string, error) {
	m.mu.Lock()
	c, ok := m.clients[deviceID]
	if !ok {
		m.mu.Unlock()
		return "", errNotFound
	}
	ps, ok := m.profiles[c.ProfileID]
	if !ok {
		m.mu.Unlock()
		return "", errProfileNotFound
	}
	p := ps.profile
	clientCopy := *c
	profCopy := *p
	host := m.cfg.WGHost
	dns := m.cfg.DNS
	allowedIPs := m.cfg.AllowedIPs
	mtu := m.cfg.MTU
	keepalive := m.cfg.PersistentKA
	m.mu.Unlock()

	switch {
	case strings.TrimSpace(allowedIPsOverride) != "":
		allowedIPs = strings.TrimSpace(allowedIPsOverride)
	case strings.TrimSpace(clientCopy.AllowedIPsOverride) != "":
		allowedIPs = strings.TrimSpace(clientCopy.AllowedIPsOverride)
	}

	portStr := strconv.Itoa(profCopy.Port)

	// ── DNS: split "1.1.1.1, 8.8.8.8" into dns1 / dns2 ────────────────
	dns1, dns2 := splitDNS(dns)

	// ── Render config with the effective DNS configuration ──────────────
	confAmnezia, err := RenderClient(ClientRenderArgs{
		Profile:    &profCopy,
		Client:     &clientCopy,
		DNS:        dns,
		MTU:        mtu,
		AllowedIPs: allowedIPs,
		Endpoint:   fmt.Sprintf("%s:%d", host, profCopy.Port),
		// Profile-level PersistentKeepalive (AWG 3.1 range) wins over the
		// server-wide integer; RenderClient resolves it.
		KeepaliveSecs: keepalive,
	})
	if err != nil {
		return "", err
	}

	allowedIPsArr := splitAllowedIPs(allowedIPs)
	mtuStr := ""
	if mtu > 0 {
		mtuStr = strconv.Itoa(mtu)
	}
	keepaliveStr := resolveKeepalive(&profCopy, keepalive)

	// I1–I5 always present in last_config (empty string when unused), matching
	// official Amnezia third-party AWG 2.0 exports.
	last := map[string]any{
		"H1":                    profCopy.H1,
		"H2":                    profCopy.H2,
		"H3":                    profCopy.H3,
		"H4":                    profCopy.H4,
		"S1":                    strconv.Itoa(profCopy.S1),
		"S2":                    strconv.Itoa(profCopy.S2),
		"S3":                    strconv.Itoa(profCopy.S3),
		"S4":                    strconv.Itoa(profCopy.S4),
		"Jc":                    strconv.Itoa(profCopy.Jc),
		"Jmin":                  strconv.Itoa(profCopy.Jmin),
		"Jmax":                  strconv.Itoa(profCopy.Jmax),
		"I1":                    profCopy.I1,
		"I2":                    profCopy.I2,
		"I3":                    profCopy.I3,
		"I4":                    profCopy.I4,
		"I5":                    profCopy.I5,
		"allowed_ips":           allowedIPsArr,
		"clientId":              "",
		"client_ip":             clientCopy.Address,
		"client_priv_key":       clientCopy.PrivateKey,
		"client_pub_key":        clientCopy.PublicKey,
		"config":                string(confAmnezia),
		"hostName":              host,
		"mtu":                   mtuStr,
		"persistent_keep_alive": keepaliveStr,
		"port":                  profCopy.Port,
		"psk_key":               clientCopy.PreSharedKey,
		"server_pub_key":        profCopy.PublicKey,
	}
	addAWG3Keys(last, &profCopy)

	lastJSON, err := json.Marshal(last)
	if err != nil {
		return "", err
	}

	// ── Outer JSON ───────────────────────────────────────────────────────
	// AWG params are also present at the awg{} top level (not just last_config)
	// so Amnezia's configurator can read them without parsing last_config.
	// protocol_version is informational — AmneziaVPN 5.x detects the real
	// generation from the markers themselves — but we report it honestly.
	awgObj := map[string]any{
		"H1":                 profCopy.H1,
		"H2":                 profCopy.H2,
		"H3":                 profCopy.H3,
		"H4":                 profCopy.H4,
		"S1":                 strconv.Itoa(profCopy.S1),
		"S2":                 strconv.Itoa(profCopy.S2),
		"S3":                 strconv.Itoa(profCopy.S3),
		"S4":                 strconv.Itoa(profCopy.S4),
		"Jc":                 strconv.Itoa(profCopy.Jc),
		"Jmin":               strconv.Itoa(profCopy.Jmin),
		"Jmax":               strconv.Itoa(profCopy.Jmax),
		"last_config":        string(lastJSON),
		"port":               portStr,
		"transport_proto":    "udp",
		"isThirdPartyConfig": true,
		"protocol_version":   protocolVersionForExport(&profCopy),
	}
	addAWG3Keys(awgObj, &profCopy)

	server := map[string]any{
		"description":      profCopy.Name,
		"hostName":         host,
		"defaultContainer": "amnezia-awg",
		"dns1":             dns1,
		"dns2":             dns2,
		"containers": []map[string]any{
			{
				"container": "amnezia-awg",
				"awg":       awgObj,
			},
		},
	}

	body, err := json.Marshal(server)
	if err != nil {
		return "", err
	}

	// ── Encode: uint32_be(uncompressed_len) + zlib(json) ────────────────
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(body))); err != nil {
		return "", err
	}
	zw, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := zw.Write(body); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return "vpn://" + base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

// splitDNS splits "1.1.1.1, 8.8.8.8" → ("1.1.1.1", "8.8.8.8").
// If only one address is present, dns2 is an empty string.
func splitDNS(dns string) (dns1, dns2 string) {
	parts := strings.SplitN(dns, ",", 2)
	dns1 = strings.TrimSpace(parts[0])
	if len(parts) > 1 {
		dns2 = strings.TrimSpace(parts[1])
	}
	return
}

func splitAllowedIPs(allowedIPs string) []string {
	parts := strings.Split(allowedIPs, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return []string{"0.0.0.0/0", "::/0"}
	}
	return result
}

// addAWG3Keys adds the AWG 3.x device params to an exported JSON object,
// under the exact key names used in the .conf. AmneziaVPN 5.x reads these
// names verbatim from both awg{} and last_config.
//
// Only keys that are actually set are added: their presence is what makes the
// client classify the config as v3, so an AWG 1.0/2.0 export must stay clean
// or an old client would be handed a generation it cannot speak.
func addAWG3Keys(dst map[string]any, p *Profile) {
	for k, v := range map[string]string{
		"HeaderProtectionKey":    p.HeaderProtectionKey,
		"ContentPaddingAddition": p.ContentPaddingAddition,
		"RekeyAfterTime":         p.RekeyAfterTime,
		"RekeyTimeout":           p.RekeyTimeout,
		"RejectAfterTime":        p.RejectAfterTime,
		"KeepaliveTimeout":       p.KeepaliveTimeout,
		"MaxHandshakeAttempts":   p.MaxHandshakeAttempts,
	} {
		if v != "" {
			dst[k] = v
		}
	}
	if p.RandomTrailers {
		dst["RandomTrailers"] = "on"
	}
	if p.DisableCookies {
		dst["DisableCookies"] = "on"
	}
}

// protocolVersionForExport maps a profile generation onto the protocol_version
// string carried in a vpn:// payload. AWG 2.0 exports keep the bare "2" they
// have always used — AmneziaVPN detects the generation from the markers
// anyway, and changing a value already in the field buys nothing.
func protocolVersionForExport(p *Profile) string {
	switch g := p.Generation(); g {
	case GenAWG2:
		return "2"
	default:
		return g
	}
}
