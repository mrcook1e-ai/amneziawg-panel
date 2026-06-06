package awg

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strconv"
	"strings"
)

// RenderAmneziaVPN builds the proprietary `vpn://` payload consumed by the
// AmneziaVPN multi-protocol client. Encoding pipeline:
//
//   JSON → qCompress (4-byte BE length + zlib) → base64url (no padding) → "vpn://" prefix
//
// The JSON schema is reverse-engineered from amnezia-client 4.x.
func RenderAmneziaVPN(a ClientRenderArgs, description string) (string, error) {
	host, portStr, _ := strings.Cut(a.Endpoint, ":")
	port, _ := strconv.Atoi(portStr)

	confBytes, err := RenderClient(a)
	if err != nil {
		return "", err
	}

	last := map[string]any{
		"H1":              a.Profile.H1,
		"H2":              a.Profile.H2,
		"H3":              a.Profile.H3,
		"H4":              a.Profile.H4,
		"Jc":              a.Profile.Jc,
		"Jmax":            a.Profile.Jmax,
		"Jmin":            a.Profile.Jmin,
		"S1":              a.Profile.S1,
		"S2":              a.Profile.S2,
		"client_ip":       a.Client.Address,
		"client_priv_key": a.Client.PrivateKey,
		"client_pub_key":  a.Client.PublicKey,
		"config":          string(confBytes),
		"hostName":        host,
		"port":            port,
		"psk_key":         a.Client.PreSharedKey,
		"server_priv_key": "",
		"server_pub_key":  a.Profile.PublicKey,
	}
	lastJSON, err := json.Marshal(last)
	if err != nil {
		return "", err
	}

	awg := map[string]any{
		"H1":              a.Profile.H1,
		"H2":              a.Profile.H2,
		"H3":              a.Profile.H3,
		"H4":              a.Profile.H4,
		"Jc":              strconv.Itoa(a.Profile.Jc),
		"Jmax":            strconv.Itoa(a.Profile.Jmax),
		"Jmin":            strconv.Itoa(a.Profile.Jmin),
		"S1":              strconv.Itoa(a.Profile.S1),
		"S2":              strconv.Itoa(a.Profile.S2),
		"last_config":     string(lastJSON),
		"port":            strconv.Itoa(port),
		"transport_proto": "udp",
	}
	if a.Profile.I1 != "" {
		awg["I1"] = a.Profile.I1
		last["I1"] = a.Profile.I1
	}
	if a.Profile.I2 != "" {
		awg["I2"] = a.Profile.I2
		last["I2"] = a.Profile.I2
	}
	if a.Profile.I3 != "" {
		awg["I3"] = a.Profile.I3
		last["I3"] = a.Profile.I3
	}
	if a.Profile.I4 != "" {
		awg["I4"] = a.Profile.I4
		last["I4"] = a.Profile.I4
	}
	if a.Profile.I5 != "" {
		awg["I5"] = a.Profile.I5
		last["I5"] = a.Profile.I5
	}

	dns1, dns2 := splitDNS(a.DNS)

	root := map[string]any{
		"containers": []any{map[string]any{
			"awg":       awg,
			"container": "amnezia-awg",
		}},
		"defaultContainer": "amnezia-awg",
		"description":      description,
		"dns1":             dns1,
		"dns2":             dns2,
		"hostName":         host,
	}

	rootJSON, err := json.Marshal(root)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(rootJSON)))
	zw, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if _, err := zw.Write(rootJSON); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}

	encoded := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(buf.Bytes())
	return "vpn://" + encoded, nil
}

func splitDNS(s string) (string, string) {
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch len(parts) {
	case 0:
		return "1.1.1.1", "1.0.0.1"
	case 1:
		return parts[0], ""
	default:
		return parts[0], parts[1]
	}
}
