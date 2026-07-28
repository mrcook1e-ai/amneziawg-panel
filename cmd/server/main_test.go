package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/mrcook1e/amneziawg-panel/internal/config"
)

func TestStartupErrorAttrs_redacts_invalid_WG_HOST_value(t *testing.T) {
	// Given
	const secret = "password-sentinel"
	t.Setenv("PORT", "51821")
	t.Setenv("WG_PORT_RANGE_START", "51820")
	t.Setenv("WG_PORT_RANGE_END", "51859")
	t.Setenv("WG_HOST", "https://user:"+secret+"@bad.invalid")
	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() error = nil, want invalid network environment error")
	}

	// When
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	logger.LogAttrs(context.Background(), slog.LevelError, "network configuration invalid", startupErrorAttrs("config", err)...)

	// Then
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("Load() error exposed secret: %q", err)
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatalf("startup log exposed secret: %q", logs.String())
	}
	if !strings.Contains(logs.String(), `"field":"WG_HOST"`) || !strings.Contains(logs.String(), `"rule":"hostname_or_ipv4"`) {
		t.Fatalf("startup log omitted safe config diagnostics: %q", logs.String())
	}
}
