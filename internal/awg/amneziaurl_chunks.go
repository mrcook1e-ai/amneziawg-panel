package awg

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// AmneziaVPNChunks splits the vpn:// payload into Amnezia-compatible chunked
// QR codes, following the exact protocol from qrCodeUtils.cpp:
//
//	amnezia-vpn/amnezia-client — client/core/utils/qrCodeUtils.cpp
//
// Wire format per QR (all big-endian, matching Qt QDataStream defaults):
//
//	magic  uint16  = 1984 (0x07C0)   — qrMagicCode constant
//	total  uint8   = total chunk count
//	idx    uint8   = 0-based chunk index
//	dlen   uint32  = length of following data  ← QDataStream QByteArray prefix
//	data   []byte  = up to 850 bytes of the compressed blob
//
// IMPORTANT: Qt's QDataStream operator<< for QByteArray writes a 4-byte
// big-endian int32 length BEFORE the array bytes. Without this prefix the
// Amnezia import controller cannot deserialise the chunk and silently rejects it.
func (m *Manager) AmneziaVPNChunks(deviceID string) ([][]byte, error) {
	url, err := m.AmneziaVPNURL(deviceID)
	if err != nil {
		return nil, err
	}

	// Strip vpn:// prefix and decode to raw compressed blob
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
		total = 255
	}

	pngs := make([][]byte, 0, total)
	for i := 0; i < total; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(blob) {
			end = len(blob)
		}
		chunk := blob[start:end]

		// Serialise exactly as Qt QDataStream does:
		//   s << qint16(magic) << quint8(total) << quint8(idx) << QByteArray(chunk)
		// QByteArray operator<< writes: int32_be(len) + raw bytes
		var pkt bytes.Buffer
		binary.Write(&pkt, binary.BigEndian, uint16(magicCode))
		pkt.WriteByte(byte(total))
		pkt.WriteByte(byte(i))
		binary.Write(&pkt, binary.BigEndian, uint32(len(chunk))) // QByteArray length prefix
		pkt.Write(chunk)

		text := base64.RawURLEncoding.EncodeToString(pkt.Bytes())
		png, err := qrcode.Encode(text, qrcode.Low, 512)
		if err != nil {
			return nil, err
		}
		pngs = append(pngs, png)
	}
	return pngs, nil
}
