package awg

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunRedactsSensitiveCommandOutputAndArguments(t *testing.T) {
	// Given
	secret := "PrivateKey = secret-key-value\ntoken=secret-token"

	// When
	err := run("/bin/sh", "-c", "printf '"+secret+"'; exit 1")

	// Then
	if err == nil {
		t.Fatal("run() error = nil, want command failure")
	}
	if strings.Contains(err.Error(), "secret-key-value") || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("run() error leaked secret: %v", err)
	}
	if !strings.Contains(err.Error(), "redacted") {
		t.Fatalf("run() error = %v, want redaction marker", err)
	}
}

func TestSanitizeKeepsOperationalAWGErrors(t *testing.T) {
	// Given — real failure text that used to be wiped by over-broad "config"/"stdin" markers
	out := []byte("Unable to modify interface: Invalid argument\nawg syncconf: fopen: No such file or directory")

	// When
	safe := sanitizeCommandOutput(out)

	// Then
	if !strings.Contains(safe, "Unable to modify interface") {
		t.Fatalf("sanitizeCommandOutput() = %q, want operational error preserved", safe)
	}
	if strings.Contains(safe, "[sensitive command output redacted]") {
		t.Fatalf("sanitizeCommandOutput() over-redacted operational error: %q", safe)
	}
}

func TestCommandCombinedOutputReturnsAfterChildExitsEvenIfGrandchildKeepsStdout(t *testing.T) {
	// Given — mimic awg-quick: child starts a background grandchild that inherits
	// stdout and stays alive. Pipe-based CombinedOutput would hang forever.
	script := `#!/bin/sh
# grandchild keeps stdout open
( sleep 60 ) &
echo started
exit 0
`
	dir := t.TempDir()
	path := dir + "/spawn.sh"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// When
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = commandCombinedOutput(path)
		close(done)
	}()

	// Then — must return promptly (not wait for grandchild)
	select {
	case <-done:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("commandCombinedOutput hung waiting on grandchild stdout — pipe leak regression")
	}
	if err != nil {
		t.Fatalf("commandCombinedOutput() err = %v", err)
	}
	if !strings.Contains(string(out), "started") {
		t.Fatalf("commandCombinedOutput() out = %q, want started", out)
	}
}

func TestSanitizeCommandOutputCapsBenignOutput(t *testing.T) {
	// Given
	output := []byte(strings.Repeat("x", maxCommandOutput+1))

	// When
	safe := sanitizeCommandOutput(output)

	// Then
	if !strings.HasSuffix(safe, " [truncated]") {
		t.Fatalf("sanitizeCommandOutput() = %q, want truncation marker", safe)
	}
	if len(safe) > maxCommandOutput+len(" [truncated]") {
		t.Fatalf("sanitizeCommandOutput() length = %d, want at most %d", len(safe), maxCommandOutput+len(" [truncated]"))
	}
}
