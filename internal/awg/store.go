package awg

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Store struct {
	dir       string
	iface     string
	jsonPath  string
	confPath  string
}

func NewStore(dir, iface string) *Store {
	return &Store{
		dir:      dir,
		iface:    iface,
		jsonPath: filepath.Join(dir, iface+".json"),
		confPath: filepath.Join(dir, iface+".conf"),
	}
}

func (s *Store) Load() (*Config, error) {
	b, err := os.ReadFile(s.jsonPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	c := &Config{Clients: map[string]*Client{}}
	if err := json.Unmarshal(b, c); err != nil {
		return nil, err
	}
	if c.Clients == nil {
		c.Clients = map[string]*Client{}
	}
	return c, nil
}

func (s *Store) Save(c *Config, serverConf []byte) error {
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return err
	}
	if err := writeAtomic(s.jsonPath, mustJSON(c), 0o660); err != nil {
		return err
	}
	return writeAtomic(s.confPath, serverConf, 0o600)
}

func (s *Store) ConfPath() string { return s.confPath }

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
