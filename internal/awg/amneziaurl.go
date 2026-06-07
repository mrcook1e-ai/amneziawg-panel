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
//   - dns1 / dns2 at root level (Amnezia substitutes $PRIMARY_DNS placeholders)
//   - AWG params both at awg{} top level and inside last_config
//   - last_config.config uses $PRIMARY_DNS / $SECONDARY_DNS placeholders
//   - isThirdPartyConfig: true (skips server-side SSH/Docker on import)
func (m *Manager) AmneziaVPNURL(deviceID string) (string, error) {
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
	host       := m.cfg.WGHost
	dns        := m.cfg.DNS
	allowedIPs := m.cfg.AllowedIPs
	mtu        := m.cfg.MTU
	keepalive  := m.cfg.PersistentKA
	m.mu.Unlock()

	portStr := strconv.Itoa(profCopy.Port)

	// ── DNS: split "1.1.1.1, 8.8.8.8" into dns1 / dns2 ────────────────
	dns1, dns2 := splitDNS(dns)

	// ── Render config with DNS placeholders for Amnezia substitution ────
	// The app reads dns1/dns2 from the outer JSON and substitutes at connect time.
	confAmnezia, err := RenderClient(ClientRenderArgs{
		Profile:    &profCopy,
		Client:     &clientCopy,
		DNS:        "$PRIMARY_DNS, $SECONDARY_DNS",
		MTU:        mtu,
		AllowedIPs: allowedIPs,
		Endpoint:   fmt.Sprintf("%s:%d", host, profCopy.Port),
		Keepalive:  keepalive,
	})
	if err != nil {
		return "", err
	}

	// ── last_config: doubly-encoded JSON inside awg{} ───────────────────
	// Field naming from configKeys.h. Numeric params are strings at this level.
	// Port is an integer here (different from awg.port which is a string).
	// allowed_ips is an array; mtu and persistent_keep_alive are strings.
	allowedIPsArr := splitAllowedIPs(allowedIPs)

	mtuStr := ""
	if mtu > 0 {
		mtuStr = strconv.Itoa(mtu)
	}

	last := map[string]any{
		"H1": profCopy.H1,
		"H2": profCopy.H2,
		"H3": profCopy.H3,
		"H4": profCopy.H4,
		"S1": strconv.Itoa(profCopy.S1),
		"S2": strconv.Itoa(profCopy.S2),
		"S3": strconv.Itoa(profCopy.S3),
		"S4": strconv.Itoa(profCopy.S4),
		"Jc":   strconv.Itoa(profCopy.Jc),
		"Jmin": strconv.Itoa(profCopy.Jmin),
		"Jmax": strconv.Itoa(profCopy.Jmax),
		"allowed_ips":           allowedIPsArr,
		"clientId":              "",
		"client_ip":             clientCopy.Address,
		"client_priv_key":       clientCopy.PrivateKey,
		"client_pub_key":        "",
		"config":                string(confAmnezia),
		"hostName":              host,
		"mtu":                   mtuStr,
		"persistent_keep_alive": strconv.Itoa(keepalive),
		"port":                  profCopy.Port, // integer — differs from outer awg.port (string)
		"psk_key":               clientCopy.PreSharedKey,
		"server_pub_key":        profCopy.PublicKey,
	}
	if profCopy.I1 != "" { last["I1"] = profCopy.I1 }
	if profCopy.I2 != "" { last["I2"] = profCopy.I2 }
	if profCopy.I3 != "" { last["I3"] = profCopy.I3 }
	if profCopy.I4 != "" { last["I4"] = profCopy.I4 }
	if profCopy.I5 != "" { last["I5"] = profCopy.I5 }

	lastJSON, err := json.Marshal(last)
	if err != nil {
		return "", err
	}

	// ── Outer JSON ───────────────────────────────────────────────────────
	// AWG params are also present at the awg{} top level (not just last_config)
	// so Amnezia's configurator can read them without parsing last_config.
	awgObj := map[string]any{
		"H1": profCopy.H1,
		"H2": profCopy.H2,
		"H3": profCopy.H3,
		"H4": profCopy.H4,
		"S1": strconv.Itoa(profCopy.S1),
		"S2": strconv.Itoa(profCopy.S2),
		"S3": strconv.Itoa(profCopy.S3),
		"S4": strconv.Itoa(profCopy.S4),
		"Jc":   strconv.Itoa(profCopy.Jc),
		"Jmin": strconv.Itoa(profCopy.Jmin),
		"Jmax": strconv.Itoa(profCopy.Jmax),
		"last_config":        string(lastJSON),
		"port":               portStr,
		"transport_proto":    "udp",
		"isThirdPartyConfig": true,
	}

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

// splitAllowedIPs parses "0.0.0.0/0, ::/0" into a string slice.
func splitAllowedIPs(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []string{"0.0.0.0/0", "::/0"}
	}
	return out
}
