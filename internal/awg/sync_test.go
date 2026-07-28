package awg

import (
	"strings"
	"testing"
)

func TestRunRedactsSensitiveCommandOutputAndArguments(t *testing.T) {
	// Given
	secret := "private_key=secret-key token=secret-token"

	// When
	err := run("/bin/sh", "-c", "printf '"+secret+"'; exit 1")

	// Then
	if err == nil {
		t.Fatal("run() error = nil, want command failure")
	}
	if strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("run() error leaked secret: %v", err)
	}
	if !strings.Contains(err.Error(), "redacted") {
		t.Fatalf("run() error = %v, want redaction marker", err)
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
