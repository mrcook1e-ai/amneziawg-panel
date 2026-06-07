package awg

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// AmneziaVPNChunks splits the vpn:// payload into Amnezia-compatible
// chunked QR codes, following the exact protocol from:
// client/core/utils/qrCodeUtils.cpp (amnezia-vpn/amnezia-client).
//
// Wire format per chunk (big-endian, matches Qt QDataStream defaults):
//
//	magic    uint16   = 1984 (0x07C0)   — qrCodeUtils::qrMagicCode
//	total    uint8    = number of chunks
//	idx      uint8    = 0-based chunk index
//	data     []byte   = up to 850 bytes of the raw compressed blob
//
// Each packet is base64url-encoded (no padding) and turned into a QR PNG.
// The Amnezia app detects the magic number, accumulates chunks, and reassembles.
func (m *Manager) AmneziaVPNChunks(deviceID string) ([][]byte, error) {
	url, err := m.AmneziaVPNURL(deviceID)
	if err != nil {
		return nil, err
	}

	// Strip vpn:// prefix and decode to raw blob
	encoded := strings.TrimPrefix(url, "vpn://")
	blob, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	const (
		magicCode = 1984
		chunkSize = 850
	)

	total := (len(blob) + chunkSize - 1) / chunkSize
	if total > 255 {
		total = 255 // quint8 max; blobs this large are pathological
	}

	pngs := make([][]byte, 0, total)
	for i := 0; i < total; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(blob) {
			end = len(blob)
		}

		var pkt bytes.Buffer
		binary.Write(&pkt, binary.BigEndian, uint16(magicCode))
		pkt.WriteByte(byte(total))
		pkt.WriteByte(byte(i))
		pkt.Write(blob[start:end])

		// Each chunk is base64url (no padding) → text QR at Low EC
		text := base64.RawURLEncoding.EncodeToString(pkt.Bytes())
		png, err := qrcode.Encode(text, qrcode.Low, 512)
		if err != nil {
			return nil, err
		}
		pngs = append(pngs, png)
	}
	return pngs, nil
}
