package stats

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrcook1e/amneziawg-panel/internal/awg"
	"github.com/mrcook1e/amneziawg-panel/internal/db"
)

func TestCollectorTickOnce_coalesces_identical_interface_failures(t *testing.T) {
	// Given
	temporaryDir := t.TempDir()
	dbStore, err := db.Open(temporaryDir)
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = dbStore.Close() })

	command := filepath.Join(temporaryDir, "awg")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("write failing command: %v", err)
	}

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	collector := &Collector{DB: dbStore, Mgr: &awg.Manager{}, Bin: command}

	// When
	collector.tickOnce(context.Background())
	collector.tickOnce(context.Background())
	if err := os.WriteFile(command, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("write successful command: %v", err)
	}
	collector.tickOnce(context.Background())

	// Then
	output := logs.String()
	if got := strings.Count(output, `"msg":"stats interface read failed"`); got != 1 {
		t.Fatalf("failure log count = %d, want 1; logs: %s", got, output)
	}
	if got := strings.Count(output, `"msg":"stats interface read recovered"`); got != 1 {
		t.Fatalf("recovery log count = %d, want 1; logs: %s", got, output)
	}
}
