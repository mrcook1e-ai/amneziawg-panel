package config

import "testing"

func TestLoad_uses_forty_profile_port_range_by_default(t *testing.T) {
	// Given
	t.Setenv("WG_PORT_RANGE_END", "")

	// When
	cfg := Load()

	// Then
	if cfg.PortRangeEnd != 51859 {
		t.Fatalf("PortRangeEnd = %d, want 51859", cfg.PortRangeEnd)
	}
}
