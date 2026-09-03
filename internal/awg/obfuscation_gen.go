package awg

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
)

// Cabinet obfuscation presets, one per AmneziaWG protocol generation. The
// parameters track the official AmneziaVPN 5.x client defaults rather than
// any home-grown tuning: matching the reference implementation is what buys
// compatibility with official clients and third-party implementations, and
// a profile that looks like every other Amnezia profile is the point.
//
// The pinned amneziawg-go v3 serves all three generations from one binary —
// with the 3.x device params unset it is byte-for-byte a 2.0/1.0 device on
// the wire.
const (
	// PresetAWG1 targets routers and old clients (Keenetic/OpenWrt/GL.iNet,
	// kernel module 1.0): fixed H values, no S3/S4, no CPS, no 3.x keys.
	PresetAWG1 = "awg1"
	// PresetAWG2 is the compatibility default: H ranges, S3/S4, official I1.
	PresetAWG2 = "awg2"
	// PresetAWG31 is the current generation: header protection, content
	// padding, randomised timers, random trailers.
	PresetAWG31 = "awg31"

	// Legacy cabinet preset names. They predate the generation model and are
	// kept as aliases so cached client bundles and old API callers keep
	// working; all three resolve to AWG 2.0, which is what they produced.
	PresetAuto    = "auto"
	PresetStealth = "stealth"
	PresetFast    = "fast"

	// DefaultPreset is used when the caller sends no preset. It stays at
	// AWG 2.0 while the 3.1 line settles: a subscriber on a pre-5.x Amnezia
	// build cannot connect to an awg31 profile at all, and the cabinet has
	// no way to know which app version they have. AWG 3.1 is offered as the
	// recommended card in the UI, and this default moves once it has been
	// through a few weeks in the field.
	DefaultPreset = PresetAWG2

	// defaultI1CPS is Amnezia's official defaultSpecialJunk1 — a compact
	// DNS-like initiation packet. The official defaults leave I2–I5 empty,
	// and so do we: an arbitrary CPS signature has to be proven against a
	// real target network before it helps anyone.
	defaultI1CPS = "<r 2><b 0x858000010001000000000669636c6f756403636f6d0000010001c00c000100010000105a00044d583737>"
)

// NormalizePreset maps a caller-supplied preset name onto a canonical one.
// Unknown or empty names fall back to DefaultPreset.
func NormalizePreset(preset string) string {
	switch preset {
	case PresetAWG1:
		return PresetAWG1
	case PresetAWG31:
		return PresetAWG31
	case PresetAWG2, PresetAuto, PresetStealth, PresetFast:
		return PresetAWG2
	default:
		return DefaultPreset
	}
}

// KeyGenFunc produces a fresh base64 32-byte key. AWG 3.1 needs one for
// HeaderProtectionKey; the other generations never call it.
type KeyGenFunc func() (string, error)

// GenerateObfuscation builds a validated ObfuscationSpec for a cabinet preset.
// genKey supplies the header protection key and is only consulted for the
// AWG 3.1 preset — pass nil when generating 1.0/2.0 profiles.
func GenerateObfuscation(preset string, genKey KeyGenFunc) (ObfuscationSpec, error) {
	switch NormalizePreset(preset) {
	case PresetAWG1:
		return genAWG1()
	case PresetAWG31:
		return genAWG31(genKey)
	default:
		return genAWG2()
	}
}

// genAWG1 builds an AWG 1.0 spec: junk train, S1/S2 only, four fixed magic
// headers. No S3/S4, no CPS, no 3.x keys — a 1.0 parser aborts on any key it
// does not know, so the conf has to stay inside the 1.0 vocabulary.
func genAWG1() (ObfuscationSpec, error) {
	jc, err := randInt(3, 6)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	s1, s2, _, _, err := randS(sBand{lo: 15, hi: 150, s4: 0}, false)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	h, err := randFixedHeaders()
	if err != nil {
		return ObfuscationSpec{}, err
	}
	return finish(ObfuscationSpec{
		Jc: jc, Jmin: 10, Jmax: 50,
		S1: s1, S2: s2,
		H1: h[0], H2: h[1], H3: h[2], H4: h[3],
	})
}

// genAWG2 builds an AWG 2.0 spec: S3/S4, magic headers as non-overlapping
// ranges, and the official DNS-mimicry I1.
func genAWG2() (ObfuscationSpec, error) {
	jc, err := randInt(4, 7)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	s1, s2, s3, s4, err := randS(sBand{lo: 15, hi: 150, s3Hi: 64, s4: 12}, true)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	h, err := randHeaderRanges()
	if err != nil {
		return ObfuscationSpec{}, err
	}
	return finish(ObfuscationSpec{
		Jc: jc, Jmin: 10, Jmax: 50,
		S1: s1, S2: s2, S3: s3, S4: s4,
		H1: h[0], H2: h[1], H3: h[2], H4: h[3],
		I1: defaultI1CPS,
	})
}

// genAWG31 builds an AWG 3.1 spec matching the official client's default
// profile. Two things differ sharply from 2.0:
//
//   - every S is at least 12, because amneziawg-go reads the ChaCha20 nonce
//     for header protection out of the first 12 bytes of the S prefix;
//   - H1–H4 go back to the plain WireGuard values 1,2,3,4. Hiding them is
//     pointless once HeaderProtectionKey encrypts the headers outright, and
//     standard values are the least remarkable thing on the wire.
func genAWG31(genKey KeyGenFunc) (ObfuscationSpec, error) {
	jc, err := randInt(4, 6)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	s1, s2, s3, s4, err := randS(sBand{lo: hpkNonceBytes, hi: 150, s3Hi: 64, s4: hpkNonceBytes}, true)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	hpk, err := generateHeaderProtectionKey(genKey)
	if err != nil {
		return ObfuscationSpec{}, err
	}
	return finish(ObfuscationSpec{
		Jc: jc, Jmin: 10, Jmax: 50,
		S1: s1, S2: s2, S3: s3, S4: s4,
		H1: "1", H2: "2", H3: "3", H4: "4",
		I1: defaultI1CPS,

		HeaderProtectionKey:    hpk,
		ContentPaddingAddition: "10-100",
		RekeyAfterTime:         "100-120",
		RekeyTimeout:           "3-7",
		RejectAfterTime:        "150-180",
		KeepaliveTimeout:       "5-15",
		MaxHandshakeAttempts:   "15-20",
		RandomTrailers:         true,
		DisableCookies:         true,
	})
}

// finish runs the generated spec through the same validation an admin-pasted
// snippet gets, so a generator bug surfaces here instead of at `awg setconf`.
func finish(spec ObfuscationSpec) (ObfuscationSpec, error) {
	if err := spec.Validate(); err != nil {
		return ObfuscationSpec{}, fmt.Errorf("generated invalid obfuscation: %w", err)
	}
	return spec, nil
}

// generateHeaderProtectionKey draws a header protection key and checks it is
// the 32 base64 bytes amneziawg-go expects, so a broken or missing awg binary
// fails here rather than at `awg setconf` with the interface already half up.
func generateHeaderProtectionKey(genKey KeyGenFunc) (string, error) {
	if genKey == nil {
		return "", fmt.Errorf("AWG 3.1 needs a header protection key generator")
	}
	key, err := genKey()
	if err != nil {
		return "", fmt.Errorf("generate header protection key: %w", err)
	}
	var probe string
	if err := parseHPKField(&probe, key); err != nil {
		return "", fmt.Errorf("generated header protection key is invalid: %w", err)
	}
	return probe, nil
}

// sBand describes the S1–S4 draw. s3Hi of 0 and s4 of 0 mean "this generation
// has no S3/S4" (AWG 1.0).
type sBand struct {
	lo, hi int // S1/S2 bounds, inclusive
	s3Hi   int // S3 upper bound; lower is lo
	s4     int // fixed S4
}

// randS draws padding sizes that satisfy the anti-collision invariants: two
// packet types whose padded sizes coincide would be trivially distinguishable
// from each other, which is the whole thing the padding exists to prevent.
func randS(b sBand, withS34 bool) (s1, s2, s3, s4 int, err error) {
	for attempt := 0; attempt < 64; attempt++ {
		s1, err = randInt(b.lo, b.hi)
		if err != nil {
			return
		}
		s2, err = randInt(b.lo, b.hi)
		if err != nil {
			return
		}
		if withS34 {
			s3, err = randInt(b.lo, b.s3Hi)
			if err != nil {
				return
			}
			s4 = b.s4
		}
		if s1 == s2 || s1+56 == s2 || s1+56 == s3 || s2+92 == s3 {
			continue
		}
		return s1, s2, s3, s4, nil
	}
	// Deterministic safe fallback with unique, invariant-satisfying sizes.
	if !withS34 {
		return b.lo, b.lo + 5, 0, 0, nil
	}
	return b.lo, b.lo + 5, b.lo + 10, b.s4, nil
}

// randFixedHeaders draws four distinct magic header values for AWG 1.0. They
// must avoid 1–4, which are the standard WireGuard message types.
func randFixedHeaders() ([4]string, error) {
	var out [4]string
	seen := map[uint64]bool{}
	for i := 0; i < 4; {
		v, err := randUint64n(0xFFFF_FFFF - 5)
		if err != nil {
			return out, err
		}
		v += 5
		if seen[v] {
			continue
		}
		seen[v] = true
		out[i] = fmt.Sprintf("%d", v)
		i++
	}
	return out, nil
}

// randHeaderRanges draws four non-overlapping magic header ranges for AWG 2.0,
// one from each quarter of the uint32 space so they cannot collide.
func randHeaderRanges() ([4]string, error) {
	var out [4]string
	bands := [4][2]uint64{
		{100_000_000, 900_000_000},
		{1_200_000_000, 2_000_000_000},
		{2_400_000_000, 3_200_000_000},
		{3_600_000_000, 4_000_000_000},
	}
	for i, b := range bands {
		r, err := randHRange(b[0], b[1], 50_000)
		if err != nil {
			return out, err
		}
		out[i] = r
	}
	return out, nil
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
