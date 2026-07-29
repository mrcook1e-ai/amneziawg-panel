package awg

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
)

// Cabinet obfuscation presets. "auto" is what almost every user picks — it
// MUST prioritise WAN handshake reliability over aggressive DPI mimicry.
//
// Ranges stay tighter than the web Architect generator. Specs are AWG 2.0:
// H ranges, S1–S4, and Amnezia's official default I1 CPS. I2–I5 remain empty;
// arbitrary CPS signatures must be manually proven on a target network.
const (
	PresetAuto    = "auto"
	PresetStealth = "stealth"
	PresetFast    = "fast"

	// defaultI1CPS is Amnezia's AWG 2.0 defaultSpecialJunk1. It is a compact
	// DNS-like initiation packet; the official defaults leave I2–I5 empty.
	defaultI1CPS = "<r 2><b 0x858000010001000000000669636c6f756403636f6d0000010001c00c000100010000105a00044d583737>"
)

// GenerateObfuscation builds a validated ObfuscationSpec for a cabinet preset.
// Unknown / empty preset defaults to auto.
func GenerateObfuscation(preset string) (ObfuscationSpec, error) {
	switch preset {
	case PresetStealth:
		return genObfuscation(obfBand{
			jcMin: 5, jcMax: 8,
			jminLo: 80, jminHi: 140,
			jmaxLo: 200, jmaxHi: 380,
			sMax: 48, s4Max: 16,
		})
	case PresetFast:
		return genObfuscation(obfBand{
			jcMin: 3, jcMax: 4,
			jminLo: 40, jminHi: 64,
			jmaxLo: 80, jmaxHi: 140,
			sMax: 24, s4Max: 4,
		})
	default: // auto
		return genObfuscation(obfBand{
			jcMin: 3, jcMax: 5,
			jminLo: 50, jminHi: 80,
			jmaxLo: 100, jmaxHi: 180,
			sMax: 40, s4Max: 8,
		})
	}
}

type obfBand struct {
	jcMin, jcMax   int
	jminLo, jminHi int
	jmaxLo, jmaxHi int
	sMax           int // S1–S3 upper (inclusive), lower is 8
	s4Max          int
}

func genObfuscation(b obfBand) (ObfuscationSpec, error) {
	jc, err := randInt(b.jcMin, b.jcMax)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	jmin, err := randInt(b.jminLo, b.jminHi)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	jmax, err := randInt(b.jmaxLo, b.jmaxHi)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	if jmax <= jmin {
		jmax = jmin + 40
	}

	s1, s2, s3, s4, err := randS(b.sMax, b.s4Max)
	if err != nil {
		return ObfuscationSpec{}, err
	}

	h1, err := randHRange(100_000_000, 900_000_000, 50_000)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	h2, err := randHRange(1_200_000_000, 2_000_000_000, 50_000)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	h3, err := randHRange(2_400_000_000, 3_200_000_000, 50_000)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	h4, err := randHRange(3_600_000_000, 4_000_000_000, 50_000)
	if err != nil {
		return ObfuscationSpec{}, err
	}

	spec := ObfuscationSpec{
		Jc: jc, Jmin: jmin, Jmax: jmax,
		S1: s1, S2: s2, S3: s3, S4: s4,
		H1: h1, H2: h2, H3: h3, H4: h4,
		I1: defaultI1CPS,
	}
	if err := spec.Validate(); err != nil {
		return ObfuscationSpec{}, fmt.Errorf("generated invalid obfuscation: %w", err)
	}
	return spec, nil
}

func randS(sMax, s4Max int) (s1, s2, s3, s4 int, err error) {
	if sMax < 16 {
		sMax = 16
	}
	if s4Max < 1 {
		s4Max = 1
	}
	for attempt := 0; attempt < 32; attempt++ {
		s1, err = randInt(8, sMax)
		if err != nil {
			return
		}
		s2, err = randInt(8, sMax)
		if err != nil {
			return
		}
		s3, err = randInt(8, sMax)
		if err != nil {
			return
		}
		s4, err = randInt(1, s4Max)
		if err != nil {
			return
		}
		if s1+56 == s2 || s1+56 == s3 || s2+92 == s3 {
			continue
		}
		return s1, s2, s3, s4, nil
	}
	// Deterministic safe fallback (unique sizes).
	return 15, 20, 10, 4, nil
}

func randHRange(loBase, hiBase, spread uint64) (string, error) {
	// Pick start in [loBase, hiBase], end = start + [1000, spread].
	span := hiBase - loBase
	if span == 0 {
		return fmt.Sprintf("%d-%d", loBase, loBase+1000), nil
	}
	off, err := randUint64n(span + 1)
	if err != nil {
		return "", err
	}
	start := loBase + off
	w, err := randUint64n(spread)
	if err != nil {
		return "", err
	}
	if w < 1000 {
		w = 1000
	}
	end := start + w
	return fmt.Sprintf("%d-%d", start, end), nil
}

func randInt(lo, hi int) (int, error) {
	if hi < lo {
		hi = lo
	}
	n := hi - lo + 1
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return lo + int(v.Int64()), nil
}

func randUint64n(n uint64) (uint64, error) {
	if n == 0 {
		return 0, nil
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b[:]) % n, nil
}
