package awg

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mrcook1e/amneziawg-panel/internal/config"
)

func TestPortIPAMMapping_preserves_default_profile_subnets(t *testing.T) {
	// Given
	mgr, err := NewManager(config.Config{
		WGPath:         t.TempDir(),
		Subnet:         "10.8.0.x",
		PortRangeStart: 51820,
		PortRangeEnd:   51859,
	})
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	tests := []struct {
		port       int
		wantIface  string
		wantSubnet string
	}{
		{port: 51820, wantIface: "awg0", wantSubnet: "10.8.0.x"},
		{port: 51859, wantIface: "awg39", wantSubnet: "10.8.39.x"},
	}

	for _, tt := range tests {
		t.Run(tt.wantIface, func(t *testing.T) {
			// When
			gotIface := mgr.portIPAM.IfaceFor(tt.port)
			gotSubnet := mgr.subnetPatternForPort(tt.port)

			// Then
			if gotIface != tt.wantIface {
				t.Fatalf("IfaceFor(%d) = %q, want %q", tt.port, gotIface, tt.wantIface)
			}
			if gotSubnet != tt.wantSubnet {
				t.Fatalf("subnetPatternForPort(%d) = %q, want %q", tt.port, gotSubnet, tt.wantSubnet)
			}
		})
	}
}

func TestManagerStartPersistedRange_starts_compatible_mapping(t *testing.T) {
	// Given
	stateDir := t.TempDir()
	logPath := filepath.Join(stateDir, "runner.log")
	runnerPath := writeRecordingRunner(t, stateDir)
	t.Setenv("RUNNER_LOG", logPath)
	mgr := newManagerWithState(t, stateDir, runnerPath, persistedMapping())

	// When
	err := mgr.Start()

	// Then
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logPath, err)
	}
	gotCalls := strings.Count(strings.TrimSpace(string(got)), "\n") + 1
	t.Logf("runner mutation count = %d", gotCalls)
	if gotCalls != 4 {
		t.Fatalf("runner mutation count = %d, want 4; calls:\n%s", gotCalls, got)
	}
}

func TestManagerStartPersistedRange_rejects_incompatible_mapping_before_runner_mutation(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
		state *Config
		rule string
	}{
		{
			name: "shifted range excludes persisted port",
			cfg:  managerConfig(t.TempDir(), "", 51821, 51859),
			state: persistedMapping(),
			rule: "port_range",
		},
		{
			name: "persisted interface does not match port",
			cfg:  managerConfig(t.TempDir(), "", 51820, 51859),
			state: persistedMappingWith(func(state *Config) { state.Profiles["profile-1"].Iface = "awg1" }),
			rule: "interface_mapping",
		},
		{
			name: "persisted server address is outside deterministic subnet",
			cfg:  managerConfig(t.TempDir(), "", 51820, 51859),
			state: persistedMappingWith(func(state *Config) { state.Profiles["profile-1"].Address = "10.8.1.1" }),
			rule: "server_subnet",
		},
		{
			name: "persisted client address is outside deterministic subnet",
			cfg:  managerConfig(t.TempDir(), "", 51820, 51859),
			state: persistedMappingWith(func(state *Config) { state.Clients["client-1"].Address = "10.8.1.2" }),
			rule: "client_subnet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			stateDir := t.TempDir()
			logPath := filepath.Join(stateDir, "runner.log")
			runnerPath := writeRecordingRunner(t, stateDir)
			t.Setenv("RUNNER_LOG", logPath)
			tt.cfg.WGPath = stateDir
			tt.cfg.AWGBin = runnerPath
			tt.cfg.AWGQuickBin = runnerPath
			mgr, err := NewManager(tt.cfg)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			if err := mgr.store.SaveState(tt.state); err != nil {
				t.Fatalf("SaveState() error = %v", err)
			}

			// When
			err = mgr.Start()

			// Then
			if !errors.Is(err, ErrPersistedProfileIncompatible) {
				t.Fatalf("Start() error = %v, want ErrPersistedProfileIncompatible", err)
			}
			t.Logf("incompatibility error = %v", err)
			var mappingErr *PersistedProfileError
			if !errors.As(err, &mappingErr) {
				t.Fatalf("Start() error = %T, want *PersistedProfileError", err)
			}
			if mappingErr.Rule != tt.rule {
				t.Fatalf("PersistedProfileError.Rule = %q, want %q", mappingErr.Rule, tt.rule)
			}
			if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("runner mutation log error = %v, want not exist", err)
			}
			if _, err := os.Stat(filepath.Join(stateDir, "awg0.conf")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("persisted profile config error = %v, want not exist", err)
			}
		})
	}
}

func newManagerWithState(t *testing.T, stateDir, runnerPath string, state *Config) *Manager {
	t.Helper()
	cfg := managerConfig(stateDir, runnerPath, 51820, 51859)
	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := mgr.store.SaveState(state); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}
	return mgr
}

func managerConfig(stateDir, runnerPath string, rangeStart, rangeEnd int) config.Config {
	return config.Config{
		WGHost:         "vpn.example.com",
		WGPath:         stateDir,
		Subnet:         "10.8.0.x",
		EgressIface:    "eth0",
		AWGBin:         runnerPath,
		AWGQuickBin:    runnerPath,
		PortRangeStart: rangeStart,
		PortRangeEnd:   rangeEnd,
	}
}

func persistedMapping() *Config {
	return &Config{
		Profiles: map[string]*Profile{
			"profile-1": {ID: "profile-1", Iface: "awg0", Port: 51820, Address: "10.8.0.1"},
		},
		Clients: map[string]*Client{
			"client-1": {ID: "client-1", ProfileID: "profile-1", Address: "10.8.0.2", Enabled: true},
		},
	}
}

func persistedMappingWith(change func(*Config)) *Config {
	state := persistedMapping()
	change(state)
	return state
}

func writeRecordingRunner(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "record-runner.sh")
	contents := "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\"$RUNNER_LOG\"\n"
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
