package awg

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrSchemaTooOld is returned by Load when state.json predates MinSchemaVersion.
// There is no in-place migration that far back: the operator must wipe
// state.json (and the rendered awgN.conf files alongside it) and recreate
// profiles.
var ErrSchemaTooOld = errors.New("state.json schema is too old — wipe state and recreate profiles")

// ErrSchemaTooNew is returned by Load when state.json was written by a newer
// binary. Loading it anyway would silently drop every field this build does
// not know about and persist that loss on the next Save, so refuse instead and
// let the operator roll forward again.
var ErrSchemaTooNew = errors.New("state.json was written by a newer version — upgrade the panel binary")

const StateFile = "state.json"

type Store struct {
	dir       string
	statePath string
}

func NewStore(dir string) *Store {
	return &Store{
		dir:       dir,
		statePath: filepath.Join(dir, StateFile),
	}
}

func (s *Store) Dir() string       { return s.dir }
func (s *Store) StatePath() string { return s.statePath }

// ConfPath returns the path of the rendered awg-quick config for an interface
// living in this store's directory.
func (s *Store) ConfPath(iface string) string {
	return filepath.Join(s.dir, iface+".conf")
}

func (s *Store) Load() (*Config, error) {
	b, err := os.ReadFile(s.statePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	c := &Config{}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	// Accept the whole supported window [MinSchemaVersion, SchemaVersion]:
	// state written by an older but still supported build loads as-is and is
	// rewritten at the current version on the next Save. A zero version is an
	// empty or hand-made file and stays tolerated, as it always was.
	if c.SchemaVersion != 0 && c.SchemaVersion < MinSchemaVersion {
		return nil, fmt.Errorf("%w (found v%d, need at least v%d)", ErrSchemaTooOld, c.SchemaVersion, MinSchemaVersion)
	}
	if c.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("%w (found v%d, this build writes v%d)", ErrSchemaTooNew, c.SchemaVersion, SchemaVersion)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]*Profile{}
	}
	if c.Clients == nil {
		c.Clients = map[string]*Client{}
	}
	return c, nil
}

// SaveState writes only the JSON state file (no .conf rendering). Use
// SaveProfileConf alongside this for the awg-quick files.
func (s *Store) SaveState(c *Config) error {
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return err
	}
	c.SchemaVersion = SchemaVersion
	return writeAtomic(s.statePath, mustJSON(c), 0o660)
}

func (s *Store) SaveProfileConf(iface string, conf []byte) error {
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return err
	}
	return writeAtomic(s.ConfPath(iface), conf, 0o600)
}

func (s *Store) RemoveProfileConf(iface string) error {
	err := os.Remove(s.ConfPath(iface))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func mustJSON(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
