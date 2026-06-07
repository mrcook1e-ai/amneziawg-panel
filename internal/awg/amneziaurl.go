package awg

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
)

// AmneziaVPNURL builds a `vpn://AAAN...` URL compatible with the official
// AmneziaVPN client (https://github.com/amnezia-vpn/amnezia-client).
//
// Wire format reverse-engineered from the official client:
//
//	url      = "vpn://" + base64url_nopad(blob)
//	blob     = uint32_be(len(json)) || zlib_deflate(json)
//
// The 4-byte length prefix is what Qt's `qCompress` emits before the standard
// zlib stream. Decode is the exact inverse — `qUncompress` reads the length,
// strips it, runs `qUncompress`/zlib over the rest, parses JSON.
//
// The JSON shape is a `ServerConfig` with a single `amnezia-awg` container.
// The container's `awg.last_config` field is itself a STRINGIFIED JSON blob
// (escaped, not nested) — this is how the official client serialises a
// per-protocol payload regardless of protocol.
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
	// Snapshot fields under the lock; rendering happens after release.
	clientCopy := *c
	profCopy := *p
	host := m.cfg.WGHost
	dns := m.cfg.DNS
	allowedIPs := m.cfg.AllowedIPs
	mtu := m.cfg.MTU
	keepalive := m.cfg.PersistentKA
	m.mu.Unlock()

	conf, err := RenderClient(ClientRenderArgs{
		Profile:    &profCopy,
		Client:     &clientCopy,
		DNS:        dns,
		MTU:        mtu,
		AllowedIPs: allowedIPs,
		Endpoint:   fmt.Sprintf("%s:%d", host, profCopy.Port),
		Keepalive:  keepalive,
	})
	if err != nil {
		return "", err
	}

	portStr := strconv.Itoa(profCopy.Port)

	// Field naming mirrors amnezia-client/client/core/utils/constants/configKeys.h.
	// Numeric AWG params are sent as strings — that's what the official client
	// emits, and importers tolerate either, so we match for max compatibility.
	last := map[string]string{
		"H1":              profCopy.H1,
		"H2":              profCopy.H2,
		"H3":              profCopy.H3,
		"H4":              profCopy.H4,
		"Jc":              strconv.Itoa(profCopy.Jc),
		"Jmin":            strconv.Itoa(profCopy.Jmin),
		"Jmax":            strconv.Itoa(profCopy.Jmax),
		"S1":              strconv.Itoa(profCopy.S1),
		"S2":              strconv.Itoa(profCopy.S2),
		"S3":              strconv.Itoa(profCopy.S3),
		"S4":              strconv.Itoa(profCopy.S4),
		"client_ip":       clientCopy.Address,
		"client_priv_key": clientCopy.PrivateKey,
		"client_pub_key":  clientCopy.PublicKey,
		"config":          string(conf),
		"hostName":        host,
		"port":            portStr,
		"psk_key":         clientCopy.PreSharedKey,
		"server_pub_key":  profCopy.PublicKey,
	}
	// Optional AWG 2.0 CPS slots — only included when set, to keep the payload
	// small and avoid confusing older importers with empty I-keys.
	if profCopy.I1 != "" {
		last["I1"] = profCopy.I1
	}
	if profCopy.I2 != "" {
		last["I2"] = profCopy.I2
	}
	if profCopy.I3 != "" {
		last["I3"] = profCopy.I3
	}
	if profCopy.I4 != "" {
		last["I4"] = profCopy.I4
	}
	if profCopy.I5 != "" {
		last["I5"] = profCopy.I5
	}

	lastJSON, err := json.Marshal(last)
	if err != nil {
		return "", err
	}

	server := map[string]any{
		"description":      profCopy.Name,
		"hostName":         host,
		"defaultContainer": "amnezia-awg",
		"containers": []map[string]any{
			{
				"container": "amnezia-awg",
				"awg": map[string]any{
					"last_config":     string(lastJSON),
					"port":            portStr,
					"transport_proto": "udp",
				},
			},
		},
	}
	body, err := json.Marshal(server)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, uint32(len(body))); err != nil {
		return "", err
	}
	// Best-compression (level 9). Qt's qCompress defaults to level 6 but emits
	// the same framing — importers only care about the zlib stream, not the
	// chosen level.
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
