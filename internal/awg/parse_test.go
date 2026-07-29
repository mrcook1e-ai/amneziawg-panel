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
