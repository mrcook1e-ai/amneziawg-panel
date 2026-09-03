package awg

import (
	"strings"
	"testing"
)

func validBaseSnippet(extra ...string) string {
	lines := []string{
		"[Interface]",
		"Jc = 5",
		"Jmin = 64",
		"Jmax = 512",
		"S1 = 15",
		"S2 = 20",
		"S3 = 10",
		"S4 = 8",
		"H1 = 100000000-100050000",
		"H2 = 1200000000-1200050000",
		"H3 = 2400000000-2400050000",
		"H4 = 3600000000-3600050000",
	}
	return strings.Join(append(lines, extra...), "\n")
}

func TestParseObfuscation_OK(t *testing.T) {
	spec, err := ParseObfuscation(validBaseSnippet(
		"I1 = <b 0xc000000001><r 32><t>",
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Jc != 5 || spec.S4 != 8 {
		t.Fatalf("parsed values wrong: %+v", spec)
	}
	if !strings.Contains(spec.I1, "<r 32>") {
		t.Fatalf("I1 not kept: %q", spec.I1)
	}
}

func TestParseObfuscation_RejectsUnsupportedCPSTag(t *testing.T) {
	// <c> is NOT in amneziawg-go v0.2.18 obfBuilders — must reject before setconf.
	_, err := ParseObfuscation(validBaseSnippet(
		"I1 = <b 0xdeadbeef><c><t><r 10>",
	))
	if err == nil {
		t.Fatal("expected error for <c> tag")
	}
	if !strings.Contains(err.Error(), "unsupported CPS tag") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestParseObfuscation_RejectsOversizedCPS(t *testing.T) {
	// 2×1000 random tags → 2000 bytes > maxCPSPacketBytes
	_, err := ParseObfuscation(validBaseSnippet(
		"I1 = <r 1000><r 1000>",
	))
	if err == nil {
		t.Fatal("expected oversized I1 error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestParseObfuscation_RejectsS4AboveRecommended(t *testing.T) {
	_, err := ParseObfuscation(strings.ReplaceAll(validBaseSnippet(), "S4 = 8", "S4 = 64"))
	if err == nil {
		t.Fatal("expected S4 range error")
	}
	if !strings.Contains(err.Error(), "S4") && !strings.Contains(err.Error(), "outside") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestParseObfuscation_RejectsSCollision(t *testing.T) {
	// S2 = S1+56
	snip := strings.ReplaceAll(validBaseSnippet(), "S1 = 15", "S1 = 10")
	snip = strings.ReplaceAll(snip, "S2 = 20", "S2 = 66")
	_, err := ParseObfuscation(snip)
	if err == nil || !strings.Contains(err.Error(), "S1+56") {
		t.Fatalf("expected S1+56 collision error, got %v", err)
	}
}

func TestEstimateCPSSize(t *testing.T) {
	// 4 hex bytes + 10 random + 4 timestamp = 18
	n := estimateCPSSize("<b 0xdeadbeef><r 10><t>")
	if n != 18 {
		t.Fatalf("got %d want 18", n)
	}
	if estimateCPSSize("") != 0 {
		t.Fatal("empty should be 0")
	}
}

const testHPK = "OjW5s9DDbnR/oPuMvHwOoHFHNXBhLUXcC0Wj4bDCOWQ="

// awg31Snippet is a valid AWG 3.1 [Interface] block: S values above the
// 12-byte header-protection nonce and standard H values.
func awg31Snippet(extra ...string) string {
	lines := []string{
		"[Interface]",
		"Jc = 5",
		"Jmin = 10",
		"Jmax = 50",
		"S1 = 100",
		"S2 = 120",
		"S3 = 30",
		"S4 = 12",
		"H1 = 1",
		"H2 = 2",
		"H3 = 3",
		"H4 = 4",
	}
	return strings.Join(append(lines, extra...), "\n")
}

func TestParseObfuscation_AWG3Keys(t *testing.T) {
	spec, err := ParseObfuscation(awg31Snippet(
		"HeaderProtectionKey = "+testHPK,
		"ContentPaddingAddition = 10-100",
		"RekeyAfterTime = 100-120",
		"RekeyTimeout = 3-7",
		"RejectAfterTime = 150-180",
		"KeepaliveTimeout = 5-15",
		"MaxHandshakeAttempts = 15-20",
		"RandomTrailers = on",
		"DisableCookies = on",
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.HeaderProtectionKey != testHPK {
		t.Fatalf("HeaderProtectionKey = %q", spec.HeaderProtectionKey)
	}
	if spec.ContentPaddingAddition != "10-100" || spec.MaxHandshakeAttempts != "15-20" {
		t.Fatalf("range fields not kept: %+v", spec)
	}
	if !spec.RandomTrailers || !spec.DisableCookies {
		t.Fatalf("bools not kept: %+v", spec)
	}

	p := &Profile{}
	applySpec(p, spec)
	if got := p.Generation(); got != GenAWG31 {
		t.Fatalf("generation = %q, want %q", got, GenAWG31)
	}
}

func TestParseObfuscation_AcceptsAndDropsDeadAWG15Keys(t *testing.T) {
	// Itime and J1-J3 are gone from every shipping implementation, but old
	// snippets still carry them and must not be rejected outright.
	spec, err := ParseObfuscation(validBaseSnippet(
		"Itime = 30",
		"J1 = <b 0xaa>",
		"J2 = <r 8>",
		"J3 = <t>",
	))
	if err != nil {
		t.Fatalf("dead AWG 1.5 keys must be tolerated, got: %v", err)
	}
	// Nothing to assert on the spec itself — the point is that the fields no
	// longer exist, so a snippet carrying them parses to a plain 2.0 profile.
	p := &Profile{}
	applySpec(p, spec)
	if got := p.Generation(); got != GenAWG2 {
		t.Fatalf("generation = %q, want %q", got, GenAWG2)
	}
}

func TestParseObfuscation_RejectsHPKWithShortS(t *testing.T) {
	// amneziawg-go reads the ChaCha20 nonce from the first 12 bytes of the S
	// prefix and refuses the UAPI set below that, leaving the interface down.
	snip := strings.ReplaceAll(awg31Snippet("HeaderProtectionKey = "+testHPK), "S4 = 12", "S4 = 8")
	_, err := ParseObfuscation(snip)
	if err == nil {
		t.Fatal("expected an error for S4 below the header-protection nonce size")
	}
	if !strings.Contains(err.Error(), "S4") || !strings.Contains(err.Error(), "HeaderProtectionKey") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestParseObfuscation_ShortSIsFineWithoutHPK(t *testing.T) {
	// The >= 12 rule only exists because of header protection.
	if _, err := ParseObfuscation(validBaseSnippet()); err != nil {
		t.Fatalf("S below 12 without a key must be fine: %v", err)
	}
}

func TestParseObfuscation_RejectsBadHPK(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"not base64", "!!!! not base64 !!!!"},
		{"wrong length", "aGVsbG8="},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseObfuscation(awg31Snippet("HeaderProtectionKey = " + tc.key))
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParseObfuscation_Bools(t *testing.T) {
	for _, tc := range []struct {
		val  string
		want bool
		ok   bool
	}{
		{"on", true, true},
		{"off", false, true},
		{"true", true, true},
		{"false", false, true},
		{"maybe", false, false},
	} {
		t.Run(tc.val, func(t *testing.T) {
			spec, err := ParseObfuscation(awg31Snippet("RandomTrailers = " + tc.val))
			if tc.ok != (err == nil) {
				t.Fatalf("err = %v, wanted ok=%v", err, tc.ok)
			}
			if tc.ok && spec.RandomTrailers != tc.want {
				t.Fatalf("RandomTrailers = %v, want %v", spec.RandomTrailers, tc.want)
			}
		})
	}
}

func TestParseObfuscation_RejectsOversizedU16Range(t *testing.T) {
	// amneziawg-tools parses these with u16_range_from_string; a larger value
	// would be silently truncated rather than rejected by awg-quick.
	_, err := ParseObfuscation(awg31Snippet("RekeyAfterTime = 100-70000"))
	if err == nil {
		t.Fatal("expected an error for a range above uint16")
	}
	if !strings.Contains(err.Error(), "65535") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestParseObfuscation_AcceptsFixedHeaders(t *testing.T) {
	// amneziawg-tools u32_range_from_string treats a bare integer as the
	// degenerate range [n, n]. AWG 1.0 and the official AWG 3.1 defaults both
	// use fixed H values, so rejecting them would rule out two generations.
	spec, err := ParseObfuscation(awg31Snippet())
	if err != nil {
		t.Fatalf("fixed H values must parse: %v", err)
	}
	if spec.H1 != "1" || spec.H4 != "4" {
		t.Fatalf("fixed H values mangled: %+v", spec)
	}
}

func TestParseObfuscation_DetectsOverlapAcrossMixedHForms(t *testing.T) {
	// A fixed value falling inside another's range is still an overlap, and
	// amneziawg-go refuses the device outright ("headers must not overlap").
	// H2 lands inside H1's range 100000000-100050000.
	snip := strings.ReplaceAll(validBaseSnippet(), "H2 = 1200000000-1200050000", "H2 = 100000500")
	_, err := ParseObfuscation(snip)
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("expected an overlap error, got %v", err)
	}
}

func TestParseObfuscation_RoundTripsDegenerateRange(t *testing.T) {
	// "5-5" and "5" mean the same thing; normalise to the shorter form the
	// tools themselves print, so generation detection stays honest.
	snip := strings.ReplaceAll(awg31Snippet(), "H1 = 1", "H1 = 1-1")
	spec, err := ParseObfuscation(snip)
	if err != nil {
		t.Fatal(err)
	}
	if spec.H1 != "1" {
		t.Fatalf("H1 = %q, want the collapsed form %q", spec.H1, "1")
	}
}
