package awg

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Limits aligned with pinned amneziawg-go v3.1 (Dockerfile AWG_GO_REF) and
// WAN path-MTU safety. See device/obf.go builders and docs.amnezia.org.
const (
	// maxS4Padding is the recommended transport-padding ceiling across AWG docs.
	maxS4Padding = 32
	// maxCPSPacketBytes rejects I* chains likely to fragment on ≤1280 paths.
	maxCPSPacketBytes = 1200
	// maxCPSTagBytes is the per-tag random-size cap in amneziawg-go.
	maxCPSTagBytes = 1000
	// hpkNonceBytes mirrors device.HeaderCipherNonceSize. With a header
	// protection key set, amneziawg-go reads the ChaCha20 nonce from the
	// first 12 bytes of the S prefix and rejects any S below that
	// ("S%d must be more then 12 to use headerProtection", uapi.go).
	hpkNonceBytes = 12
	// hpkKeyBytes is the raw length of a base64 HeaderProtectionKey.
	hpkKeyBytes = 32
	// maxU16Range caps the 3.x range-valued device params: amneziawg-tools
	// parses them with u16_range_from_string (type.c).
	maxU16Range = math.MaxUint16
)

// cpsTagSupported lists tags registered in amneziawg-go v3.1 newObfChain
// (device/obf.go obfBuilders — unchanged since v0.2.x). Notably missing:
// "c" (counter) — external generators may emit it; we reject it so
// awg setconf does not fail with "unknown tag".
var cpsTagSupported = map[string]bool{
	"b": true, "t": true, "r": true, "rc": true, "rd": true,
	"d": true, "ds": true, "dz": true,
}

var (
	cpsTagRe   = regexp.MustCompile(`<\s*([a-zA-Z]+)`)
	cpsBytesRe = regexp.MustCompile(`(?i)<\s*b\s+0x([0-9a-f]+)\s*>`)
	cpsRandRe  = regexp.MustCompile(`<\s*(?:r|rc|rd)\s+(\d+)\s*>`)
	cpsFixedRe = regexp.MustCompile(`<\s*(?:t|c)\s*>`)
)

// ObfuscationSpec is the parsed result of an admin-pasted or cabinet-generated
// AmneziaWG [Interface] snippet. The AWG 3.x fields are optional — empty means
// "not set", which reproduces AWG 2.0 behaviour exactly.
type ObfuscationSpec struct {
	Jc, Jmin, Jmax     int
	S1, S2, S3, S4     int
	H1, H2, H3, H4     string // "n" (fixed) or "min-max" range, both inclusive
	I1, I2, I3, I4, I5 string

	HeaderProtectionKey    string
	ContentPaddingAddition string
	RekeyAfterTime         string
	RekeyTimeout           string
	RejectAfterTime        string
	KeepaliveTimeout       string
	MaxHandshakeAttempts   string
	RandomTrailers         bool
	DisableCookies         bool
}

// ParseObfuscation reads a free-form snippet (the [Interface] block from an
// AmneziaWG config — section headers, comments, server/client-specific fields
// are all tolerated and ignored) and returns the obfuscation parameters.
//
// Required: Jc, Jmin, Jmax, S1..S2, H1..H4. S3/S4, I*, and the AWG 3.x keys
// are optional. Duplicates of the same key are an error.
//
// Itime and J1-J3 are still accepted and silently dropped: they were part of
// the abandoned AWG 1.5 beta and exist in no shipping implementation, but old
// snippets in circulation still carry them and should not be rejected.
func ParseObfuscation(snippet string) (ObfuscationSpec, error) {
	var spec ObfuscationSpec
	seen := map[string]bool{}

	sc := bufio.NewScanner(strings.NewReader(snippet))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		keyU := strings.ToUpper(key)

		if isIgnoredKey(keyU) {
			continue
		}
		if seen[keyU] {
			return ObfuscationSpec{}, fmt.Errorf("duplicate field %q", key)
		}
		seen[keyU] = true

		if err := applyField(&spec, keyU, val); err != nil {
			return ObfuscationSpec{}, fmt.Errorf("field %s: %w", key, err)
		}
	}
	if err := sc.Err(); err != nil {
		return ObfuscationSpec{}, err
	}

	if err := spec.Validate(); err != nil {
		return ObfuscationSpec{}, err
	}
	return spec, nil
}

func isIgnoredKey(keyU string) bool {
	switch keyU {
	case "PRIVATEKEY", "PUBLICKEY", "PRESHAREDKEY",
		"ADDRESS", "LISTENPORT", "DNS", "MTU",
		"ALLOWEDIPS", "ENDPOINT", "PERSISTENTKEEPALIVE",
		"POSTUP", "POSTDOWN", "PREUP", "PREDOWN",
		"FWMARK", "TABLE", "SAVECONFIG", "ADVANCEDSECURITY",
		// AWG 1.5 beta leftovers: dropped from amneziawg-go and
		// amneziawg-tools alike. Tolerated in input, never stored.
		"ITIME", "J1", "J2", "J3":
		return true
	}
	return false
}

func applyField(s *ObfuscationSpec, keyU, val string) error {
	switch keyU {
	case "JC":
		return parseIntField(&s.Jc, val, 0, 128)
	case "JMIN":
		return parseIntField(&s.Jmin, val, 0, 1280)
	case "JMAX":
		return parseIntField(&s.Jmax, val, 0, 1280)
	case "S1":
		return parseIntField(&s.S1, val, 0, 1280)
	case "S2":
		return parseIntField(&s.S2, val, 0, 1280)
	case "S3":
		return parseIntField(&s.S3, val, 0, 1280)
	case "S4":
		// Hard-cap at recommended 32 so transport frames stay MTU-friendly.
		return parseIntField(&s.S4, val, 0, maxS4Padding)
	case "H1":
		return parseU32RangeField(&s.H1, val)
	case "H2":
		return parseU32RangeField(&s.H2, val)
	case "H3":
		return parseU32RangeField(&s.H3, val)
	case "H4":
		return parseU32RangeField(&s.H4, val)
	case "I1":
		s.I1 = val
	case "I2":
		s.I2 = val
	case "I3":
		s.I3 = val
	case "I4":
		s.I4 = val
	case "I5":
		s.I5 = val
	case "HEADERPROTECTIONKEY":
		return parseHPKField(&s.HeaderProtectionKey, val)
	case "CONTENTPADDINGADDITION":
		return parseU16RangeField(&s.ContentPaddingAddition, val)
	case "REKEYAFTERTIME":
		return parseU16RangeField(&s.RekeyAfterTime, val)
	case "REKEYTIMEOUT":
		return parseU16RangeField(&s.RekeyTimeout, val)
	case "REJECTAFTERTIME":
		return parseU16RangeField(&s.RejectAfterTime, val)
	case "KEEPALIVETIMEOUT":
		return parseU16RangeField(&s.KeepaliveTimeout, val)
	case "MAXHANDSHAKEATTEMPTS":
		return parseU16RangeField(&s.MaxHandshakeAttempts, val)
	case "RANDOMTRAILERS":
		return parseBoolField(&s.RandomTrailers, val)
	case "DISABLECOOKIES":
		return parseBoolField(&s.DisableCookies, val)
	default:
		// Unknown key — silently ignore so future protocol additions don't
		// break snippet upload. Lint at the UI layer if strictness is needed.
	}
	return nil
}

func parseIntField(dst *int, val string, lo, hi int) error {
	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("not an integer: %q", val)
	}
	if n < lo || n > hi {
		return fmt.Errorf("%d outside [%d..%d]", n, lo, hi)
	}
	*dst = n
	return nil
}

// parseBoolField accepts the on/off spelling amneziawg-tools writes plus the
// usual truthy synonyms its parse_bool() understands.
func parseBoolField(dst *bool, val string) error {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "on", "true", "yes", "1":
		*dst = true
	case "off", "false", "no", "0":
		*dst = false
	default:
		return fmt.Errorf("expected on/off, got %q", val)
	}
	return nil
}

// parseHPKField validates a base64 32-byte header protection key (awg genkey).
func parseHPKField(dst *string, val string) error {
	val = strings.TrimSpace(val)
	raw, err := base64.StdEncoding.DecodeString(val)
	if err != nil {
		return fmt.Errorf("not valid base64: %w", err)
	}
	if len(raw) != hpkKeyBytes {
		return fmt.Errorf("must decode to %d bytes, got %d", hpkKeyBytes, len(raw))
	}
	*dst = val
	return nil
}

// parseU32RangeField accepts "n" or "min-max", matching amneziawg-tools
// u32_range_from_string (src/type.c): a bare integer is a fixed value, i.e.
// the range [n, n]. AWG 1.0 and the official AWG 3.1 defaults use fixed H
// values; AWG 2.0 uses ranges.
func parseU32RangeField(dst *string, val string) error {
	return parseRangeField(dst, val, math.MaxUint32)
}

// parseU16RangeField is the same grammar bounded to uint16, which is what
// amneziawg-tools uses for the AWG 3.x device params (u16_range_from_string).
func parseU16RangeField(dst *string, val string) error {
	return parseRangeField(dst, val, maxU16Range)
}

func parseRangeField(dst *string, val string, max uint64) error {
	r, err := parseUintRange(val, max)
	if err != nil {
		return err
	}
	*dst = formatUintRange(r)
	return nil
}

// Validate enforces the AmneziaWG invariants. Run after parsing, and again on
// every generated spec so the two paths cannot diverge.
func (s ObfuscationSpec) Validate() error {
	if s.Jmax > 0 && s.Jmax <= s.Jmin {
		return fmt.Errorf("Jmax (%d) must be greater than Jmin (%d)", s.Jmax, s.Jmin)
	}
	if s.S1 == 0 && s.S2 == 0 && s.S3 == 0 && s.S4 == 0 {
		return fmt.Errorf("S1..S4 must be set")
	}
	if s.S1+56 == s.S2 {
		return fmt.Errorf("S1+56 must not equal S2 (init/response would collide)")
	}
	if s.S1+56 == s.S3 {
		return fmt.Errorf("S1+56 must not equal S3 (init/cookie would collide)")
	}
	if s.S2+92 == s.S3 {
		return fmt.Errorf("S2+92 must not equal S3 (response/cookie would collide)")
	}
	if s.S4 > maxS4Padding {
		return fmt.Errorf("S4 %d exceeds recommended max %d", s.S4, maxS4Padding)
	}

	hs := [4]string{s.H1, s.H2, s.H3, s.H4}
	hn := [4]string{"H1", "H2", "H3", "H4"}
	for i, h := range hs {
		if h == "" {
			return fmt.Errorf("%s is required", hn[i])
		}
	}
	ranges, err := parseHRanges(hs, hn)
	if err != nil {
		return err
	}
	for i := 0; i < 4; i++ {
		for j := i + 1; j < 4; j++ {
			if rangesOverlap(ranges[i], ranges[j]) {
				return fmt.Errorf("%s and %s ranges overlap", hn[i], hn[j])
			}
		}
	}

	for _, slot := range []struct {
		name, val string
	}{
		{"I1", s.I1}, {"I2", s.I2}, {"I3", s.I3}, {"I4", s.I4}, {"I5", s.I5},
	} {
		if err := validateCPSChain(slot.name, slot.val); err != nil {
			return err
		}
	}

	return s.validateAWG3()
}

// validateAWG3 checks the AWG 3.x additions. All are optional; each is only
// checked when present.
func (s ObfuscationSpec) validateAWG3() error {
	if s.HeaderProtectionKey != "" {
		var probe string
		if err := parseHPKField(&probe, s.HeaderProtectionKey); err != nil {
			return fmt.Errorf("HeaderProtectionKey: %w", err)
		}
		// amneziawg-go reads the ChaCha20 nonce out of the S prefix, so
		// every S must leave room for it or the UAPI set is rejected and
		// the interface never comes up.
		for i, sv := range [4]int{s.S1, s.S2, s.S3, s.S4} {
			if sv < hpkNonceBytes {
				return fmt.Errorf("S%d must be at least %d when HeaderProtectionKey is set (got %d)",
					i+1, hpkNonceBytes, sv)
			}
		}
	}

	for _, f := range []struct{ name, val string }{
		{"ContentPaddingAddition", s.ContentPaddingAddition},
		{"RekeyAfterTime", s.RekeyAfterTime},
		{"RekeyTimeout", s.RekeyTimeout},
		{"RejectAfterTime", s.RejectAfterTime},
		{"KeepaliveTimeout", s.KeepaliveTimeout},
		{"MaxHandshakeAttempts", s.MaxHandshakeAttempts},
	} {
		if f.val == "" {
			continue
		}
		if _, err := parseUintRange(f.val, maxU16Range); err != nil {
			return fmt.Errorf("%s: %w", f.name, err)
		}
	}
	return nil
}

// validateCPSChain checks tag vocabulary and estimated expanded size against
// the pinned amneziawg-go parser. Empty chains are fine (slot unused).
func validateCPSChain(name, spec string) error {
	if strings.TrimSpace(spec) == "" {
		return nil
	}
	for _, r := range spec {
		if r < 0x20 || r == 0x7f || strings.ContainsRune("\"'`$;\\", r) {
			return fmt.Errorf("%s contains an unsafe character", name)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains a control character", name)
		}
	}
	for _, m := range cpsTagRe.FindAllStringSubmatch(spec, -1) {
		tag := m[1]
		if !cpsTagSupported[tag] {
			return fmt.Errorf("%s: unsupported CPS tag <%s> (pinned amneziawg-go supports b/t/r/rc/rd/d/ds/dz)", name, tag)
		}
	}
	if n := estimateCPSSize(spec); n > maxCPSPacketBytes {
		return fmt.Errorf("%s estimated size %d exceeds %d bytes (WAN fragmentation risk)", name, n, maxCPSPacketBytes)
	}
	return nil
}

// estimateCPSSize returns an upper bound on the UDP payload produced by a CPS
// chain.
func estimateCPSSize(spec string) int {
	n := 0
	for _, m := range cpsBytesRe.FindAllStringSubmatch(spec, -1) {
		n += len(m[1]) / 2
	}
	for _, m := range cpsRandRe.FindAllStringSubmatch(spec, -1) {
		v, _ := strconv.Atoi(m[1])
		if v > maxCPSTagBytes {
			v = maxCPSTagBytes
		}
		if v > 0 {
			n += v
		}
	}
	n += 4 * len(cpsFixedRe.FindAllStringIndex(spec, -1))
	return n
}

type uintRange struct{ lo, hi uint64 }

// parseUintRange accepts "n" (fixed) or "lo-hi" (inclusive), bounded by max.
func parseUintRange(val string, max uint64) (uintRange, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return uintRange{}, fmt.Errorf("empty value")
	}
	dash := strings.IndexByte(val, '-')
	if dash < 0 {
		n, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return uintRange{}, fmt.Errorf("expected a number or min-max range, got %q", val)
		}
		if n > max {
			return uintRange{}, fmt.Errorf("%d exceeds maximum %d", n, max)
		}
		return uintRange{n, n}, nil
	}
	lo, err := strconv.ParseUint(strings.TrimSpace(val[:dash]), 10, 64)
	if err != nil {
		return uintRange{}, fmt.Errorf("invalid range min: %w", err)
	}
	hi, err := strconv.ParseUint(strings.TrimSpace(val[dash+1:]), 10, 64)
	if err != nil {
		return uintRange{}, fmt.Errorf("invalid range max: %w", err)
	}
	if lo > hi {
		return uintRange{}, fmt.Errorf("range min > max (%d > %d)", lo, hi)
	}
	if hi > max {
		return uintRange{}, fmt.Errorf("range max %d exceeds maximum %d", hi, max)
	}
	return uintRange{lo, hi}, nil
}

// formatUintRange round-trips a range the way amneziawg-tools prints it
// (u32_range_to_string): a degenerate range collapses back to a bare number.
func formatUintRange(r uintRange) string {
	if r.lo == r.hi {
		return strconv.FormatUint(r.lo, 10)
	}
	return fmt.Sprintf("%d-%d", r.lo, r.hi)
}

// isRangeValue reports whether a stored H value is a true "lo-hi" range as
// opposed to a fixed number. Used to tell AWG 2.0 profiles from 1.0 ones.
func isRangeValue(v string) bool {
	return strings.ContainsRune(v, '-')
}

func parseHRanges(hs [4]string, hn [4]string) ([4]uintRange, error) {
	var out [4]uintRange
	for i, s := range hs {
		r, err := parseUintRange(s, math.MaxUint32)
		if err != nil {
			return out, fmt.Errorf("%s: %w", hn[i], err)
		}
		out[i] = r
	}
	return out, nil
}

func rangesOverlap(a, b uintRange) bool {
	return a.lo <= b.hi && b.lo <= a.hi
}
