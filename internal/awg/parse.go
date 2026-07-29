package awg

import (
	"bufio"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Limits aligned with pinned amneziawg-go v0.2.18 (Dockerfile AWG_GO_REF) and
// WAN path-MTU safety. See device/obf.go builders and docs.amnezia.org.
const (
	// maxS4Padding is the recommended transport-padding ceiling across AWG docs.
	maxS4Padding = 32
	// maxCPSPacketBytes rejects I* chains likely to fragment on ≤1280 paths.
	maxCPSPacketBytes = 1200
	// maxCPSTagBytes is the per-tag random-size cap in amneziawg-go.
	maxCPSTagBytes = 1000
)

// cpsTagSupported lists tags registered in amneziawg-go v0.2.18 newObfChain.
// Notably missing: "c" (counter) — Architect may emit it; we reject it so
// awg setconf does not fail with "unknown tag".
var cpsTagSupported = map[string]bool{
	"b": true, "t": true, "r": true, "rc": true, "rd": true,
	"d": true, "ds": true, "dz": true,
}

var (
	cpsTagRe  = regexp.MustCompile(`<\s*([a-zA-Z]+)`)
	cpsBytesRe = regexp.MustCompile(`(?i)<\s*b\s+0x([0-9a-f]+)\s*>`)
	cpsRandRe  = regexp.MustCompile(`<\s*(?:r|rc|rd)\s+(\d+)\s*>`)
	cpsFixedRe = regexp.MustCompile(`<\s*(?:t|c)\s*>`)
)

// ObfuscationSpec is the parsed result of an admin-pasted or cabinet-generated
// AWG 2.0 [Interface] snippet.
type ObfuscationSpec struct {
	Jc, Jmin, Jmax     int
	S1, S2, S3, S4     int
	H1, H2, H3, H4     string // "min-max" range, both bounds inclusive
	I1, I2, I3, I4, I5 string
	J1, J2, J3         string
	Itime              int // 0 = CPS chain disabled
}

// ParseObfuscation reads a free-form snippet (the [Interface] block from an
// AWG 2.0 config — section headers, comments, server/client-specific fields
// are all tolerated and ignored) and returns the obfuscation parameters.
//
// Required: Jc, Jmin, Jmax, S1..S4, H1..H4. Itime defaults to 0. I*/J* are
// optional. Duplicates of the same key are an error.
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
		"FWMARK", "TABLE", "SAVECONFIG":
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
		return parseRangeField(&s.H1, val)
	case "H2":
		return parseRangeField(&s.H2, val)
	case "H3":
		return parseRangeField(&s.H3, val)
	case "H4":
		return parseRangeField(&s.H4, val)
	case "ITIME":
		return parseIntField(&s.Itime, val, 0, 86400)
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
	case "J1":
		s.J1 = val
	case "J2":
		s.J2 = val
	case "J3":
		s.J3 = val
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

// parseRangeField accepts only "min-max" (both required). Single integers are
// rejected — AWG 2.0 means ranges, by design we don't paper over old formats.
func parseRangeField(dst *string, val string) error {
	dash := strings.IndexByte(val, '-')
	if dash < 0 {
		return fmt.Errorf("expected min-max range, got %q", val)
	}
	lo, err := strconv.ParseUint(strings.TrimSpace(val[:dash]), 10, 32)
	if err != nil {
		return fmt.Errorf("invalid range min: %w", err)
	}
	hi, err := strconv.ParseUint(strings.TrimSpace(val[dash+1:]), 10, 32)
	if err != nil {
		return fmt.Errorf("invalid range max: %w", err)
	}
	if lo > hi {
		return fmt.Errorf("range min > max (%d > %d)", lo, hi)
	}
	if hi > math.MaxUint32 {
		return fmt.Errorf("range max %d exceeds uint32", hi)
	}
	*dst = fmt.Sprintf("%d-%d", lo, hi)
	return nil
}

// Validate enforces the AWG 2.0 invariants. Run after parsing.
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

	hs := [4]string{s.H1, s.H2, s.H3, s.H4}
	hn := [4]string{"H1", "H2", "H3", "H4"}
	for i, h := range hs {
		if h == "" {
			return fmt.Errorf("%s is required", hn[i])
		}
	}
	ranges, err := parseHRanges(hs)
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
// chain (same accounting as web/src/utils/generator.ts estimateCPSSize).
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

func parseHRanges(hs [4]string) ([4]uintRange, error) {
	var out [4]uintRange
	for i, s := range hs {
		dash := strings.IndexByte(s, '-')
		lo, _ := strconv.ParseUint(s[:dash], 10, 64)
		hi, _ := strconv.ParseUint(s[dash+1:], 10, 64)
		out[i] = uintRange{lo, hi}
	}
	return out, nil
}

func rangesOverlap(a, b uintRange) bool {
	return a.lo <= b.hi && b.lo <= a.hi
}
