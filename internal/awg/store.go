package awg

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

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
