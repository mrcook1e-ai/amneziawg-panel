package awg

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeState(t *testing.T, dir string, raw string) *Store {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, StateFile), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return NewStore(dir)
}

// TestStoreLoad_AcceptsPreviousSchema is the upgrade gate: a deployment
// running v4 state must survive the AWG 3.1 binary rolling out. The 3.x
// profile fields are all optional, so there is nothing to migrate — but a
// version bump with a strict equality check would take every install down.
func TestStoreLoad_AcceptsPreviousSchema(t *testing.T) {
	// A v4 profile, including the AWG 1.5 fields this build no longer knows.
	raw := `{
	  "schemaVersion": 4,
	  "profiles": {
	    "p1": {
	      "id": "p1", "name": "old", "iface": "awg0", "port": 51820,
	      "privateKey": "priv", "publicKey": "pub", "address": "10.8.0.1",
	      "jc": 5, "jmin": 64, "jmax": 512,
	      "s1": 15, "s2": 20, "s3": 10, "s4": 8,
	      "h1": "100-200", "h2": "300-400", "h3": "500-600", "h4": "700-800",
	      "i1": "<r 2>", "j1": "<b 0xaa>", "itime": 30
	    }
	  },
	  "clients": {}
	}`
	c, err := writeState(t, t.TempDir(), raw).Load()
	if err != nil {
		t.Fatalf("v4 state must load on a v5 binary: %v", err)
	}
	p := c.Profiles["p1"]
	if p == nil {
		t.Fatal("profile lost on load")
	}
	if p.S1 != 15 || p.H1 != "100-200" || p.I1 != "<r 2>" {
		t.Fatalf("v4 fields mangled: %+v", p)
	}
	// The dead AWG 1.5 keys are simply unknown JSON fields now.
	if got := p.Generation(); got != GenAWG2 {
		t.Fatalf("generation = %q, want %q", got, GenAWG2)
	}
}

// TestStoreSave_UpgradesSchemaVersion pins the other half of the upgrade: a
// loaded v4 store is written back as v5, so the bump actually happens.
func TestStoreSave_UpgradesSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	s := writeState(t, dir, `{"schemaVersion": 4, "profiles": {}, "clients": {}}`)
	c, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveState(c); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(s.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.SchemaVersion != SchemaVersion {
		t.Fatalf("saved schemaVersion = %d, want %d", out.SchemaVersion, SchemaVersion)
	}
}

func TestStoreLoad_RejectsTooOldSchema(t *testing.T) {
	_, err := writeState(t, t.TempDir(), `{"schemaVersion": 3, "profiles": {}}`).Load()
	if !errors.Is(err, ErrSchemaTooOld) {
		t.Fatalf("err = %v, want ErrSchemaTooOld", err)
	}
}

// TestStoreLoad_RejectsNewerSchema guards against downgrade corruption: an
// older binary loading newer state would drop every field it does not know
// and persist that loss on the next Save.
func TestStoreLoad_RejectsNewerSchema(t *testing.T) {
	_, err := writeState(t, t.TempDir(), `{"schemaVersion": 6, "profiles": {}}`).Load()
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("err = %v, want ErrSchemaTooNew", err)
	}
}

func TestStoreLoad_MissingFileIsNotAnError(t *testing.T) {
	c, err := NewStore(t.TempDir()).Load()
	if err != nil {
		t.Fatalf("a fresh install must not error: %v", err)
	}
	if c != nil {
		t.Fatalf("expected a nil config for a missing state file, got %+v", c)
	}
}

// TestStoreLoad_PersistsAWG31Profile round-trips a full AWG 3.1 profile: the
// 3.x fields are what the schema bump exists for, so losing one on save would
// silently downgrade a live interface at the next resync.
func TestStoreLoad_PersistsAWG31Profile(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	want := &Profile{
		ID: "p1", Name: "n", Iface: "awg0", Port: 51820,
		PrivateKey: "priv", PublicKey: "pub", Address: "10.8.0.1",
		Jc: 5, Jmin: 10, Jmax: 50,
		S1: 100, S2: 120, S3: 30, S4: 12,
		H1: "1", H2: "2", H3: "3", H4: "4",
		HeaderProtectionKey:    "OjW5s9DDbnR/oPuMvHwOoHFHNXBhLUXcC0Wj4bDCOWQ=",
		ContentPaddingAddition: "10-100",
		RekeyAfterTime:         "100-120",
		RekeyTimeout:           "3-7",
		RejectAfterTime:        "150-180",
		KeepaliveTimeout:       "5-15",
		MaxHandshakeAttempts:   "15-20",
		RandomTrailers:         true,
		DisableCookies:         true,
		PersistentKeepalive:    "25-35",
	}
	if err := s.SaveState(&Config{Profiles: map[string]*Profile{"p1": want}, Clients: map[string]*Client{}}); err != nil {
		t.Fatal(err)
	}
	c, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	got := c.Profiles["p1"]
	if *got != *want {
		t.Fatalf("profile did not survive the round trip:\ngot  %+v\nwant %+v", got, want)
	}
	if got.Generation() != GenAWG31 {
		t.Fatalf("generation = %q, want %q", got.Generation(), GenAWG31)
	}
}
