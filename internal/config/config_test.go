package config

import (
	"errors"
	"strings"
	"testing"
)

func TestLoad_uses_forty_profile_port_range_by_default(t *testing.T) {
	// Given
	setValidNetworkEnv(t)
	t.Setenv("WG_PORT_RANGE_START", "")
	t.Setenv("WG_PORT_RANGE_END", "")

	// When
	cfg, err := Load()

	// Then
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PortRangeEnd != 51859 {
		t.Fatalf("PortRangeEnd = %d, want 51859", cfg.PortRangeEnd)
	}
}

func TestLoad_rejects_invalid_network_environment(t *testing.T) {
	tests := []struct {
		name      string
		nameEnv   string
		value     string
		wantField string
		wantRule  string
	}{
		{name: "malformed HTTP port", nameEnv: "PORT", value: "not-a-number", wantField: "PORT", wantRule: "integer"},
		{name: "host with scheme", nameEnv: "WG_HOST", value: "https://vpn.example.com", wantField: "WG_HOST", wantRule: "hostname_or_ipv4"},
		{name: "host with embedded port", nameEnv: "WG_HOST", value: "vpn.example.com:51820", wantField: "WG_HOST", wantRule: "hostname_or_ipv4"},
		{name: "inverted UDP range", nameEnv: "WG_PORT_RANGE_START", value: "51860", wantField: "WG_PORT_RANGE", wantRule: "start_not_after_end"},
		{name: "HTTP port above maximum", nameEnv: "PORT", value: "65536", wantField: "PORT", wantRule: "port_range"},
		{name: "UDP range exceeds subnet capacity", nameEnv: "WG_PORT_RANGE_END", value: "52076", wantField: "WG_PORT_RANGE", wantRule: "capacity"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			setValidNetworkEnv(t)
			t.Setenv(tt.nameEnv, tt.value)

			// When
			_, err := Load()

			// Then
			if err == nil {
				t.Fatal("Load() error = nil, want invalid network environment error")
			}
			if !errors.Is(err, ErrInvalidNetworkConfig) {
				t.Fatalf("Load() error = %v, want ErrInvalidNetworkConfig", err)
			}
			var envErr *EnvironmentError
			if !errors.As(err, &envErr) {
				t.Fatalf("Load() error = %T, want *EnvironmentError", err)
			}
			if envErr.Field != tt.wantField {
				t.Fatalf("EnvironmentError.Field = %q, want %q", envErr.Field, tt.wantField)
			}
			if envErr.Rule != tt.wantRule {
				t.Fatalf("EnvironmentError.Rule = %q, want %q", envErr.Rule, tt.wantRule)
			}
		})
	}
}

func TestLoad_trims_valid_network_host(t *testing.T) {
	// Given
	setValidNetworkEnv(t)
	t.Setenv("WG_HOST", "  vpn.example.com  ")

	// When
	cfg, err := Load()

	// Then
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.WGHost != "vpn.example.com" {
		t.Fatalf("WGHost = %q, want %q", cfg.WGHost, "vpn.example.com")
	}
}

func TestLoad_redacts_invalid_WG_HOST_value_from_error(t *testing.T) {
	// Given
	const secret = "password-sentinel"
	setValidNetworkEnv(t)
	t.Setenv("WG_HOST", "https://user:"+secret+"@bad.invalid")

	// When
	_, err := Load()

	// Then
	if err == nil {
		t.Fatal("Load() error = nil, want invalid network environment error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("Load() error exposed secret: %q", err)
	}
}

func setValidNetworkEnv(t *testing.T) {
	t.Helper()
	t.Setenv("WG_HOST", "vpn.example.com")
	t.Setenv("PORT", "51821")
	t.Setenv("WG_PORT_RANGE_START", "51820")
	t.Setenv("WG_PORT_RANGE_END", "51859")
}
