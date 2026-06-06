package awg

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mrcook1e/amneziawg-panel/internal/config"
)

// randomMagic returns a random uint32 string for H1-H4. AmneziaWG clients
// reject obvious placeholders like "1" / "2" — these must be large random
// uint32 values, unique per server.
func randomMagic() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	v := binary.BigEndian.Uint32(b[:])
	if v < 1_000_000 {
		v += 1_000_000
	}
	return strconv.FormatUint(uint64(v), 10)
}

func uniqueMagic() (string, string, string, string) {
	for {
		a, b, c, d := randomMagic(), randomMagic(), randomMagic(), randomMagic()
		if a != b && a != c && a != d && b != c && b != d && c != d {
			return a, b, c, d
		}
	}
}

type Manager struct {
	cfg    config.Config
	store  *Store
	keys   Keys
	runner Runner
	ipam   *IPAM

	mu  sync.Mutex
	cur *Config
}

func NewManager(cfg config.Config) (*Manager, error) {
	ipam, err := NewIPAM(cfg.Subnet)
	if err != nil {
		return nil, err
	}
	return &Manager{
		cfg:    cfg,
		store:  NewStore(cfg.WGPath, cfg.Interface),
		keys:   Keys{Bin: cfg.AWGBin},
		runner: Runner{AWGBin: cfg.AWGBin, AWGQuickBin: cfg.AWGQuickBin, Iface: cfg.Interface},
		ipam:   ipam,
	}, nil
}

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cfg.WGHost == "" {
		return errors.New("WG_HOST is not set")
	}

	c, err := m.store.Load()
	if err != nil {
		return err
	}
	if c == nil {
		c, err = m.bootstrap()
		if err != nil {
			return err
		}
	}
	m.cur = c
	if err := m.persistLocked(); err != nil {
		return err
	}
	_ = m.runner.Down()
	if err := m.runner.Up(); err != nil {
		return fmt.Errorf("awg-quick up: %w", err)
	}
	return m.runner.SyncConf()
}

func (m *Manager) Shutdown() error { return m.runner.Down() }

func (m *Manager) bootstrap() (*Config, error) {
	priv, err := m.keys.GenPrivate()
	if err != nil {
		return nil, err
	}
	pub, err := m.keys.Public(priv)
	if err != nil {
		return nil, err
	}
	o := m.cfg.Obf
	h1, h2, h3, h4 := o.H1, o.H2, o.H3, o.H4
	// Treat placeholder defaults ("1".."4") as a signal to roll random magic.
	if h1 == "1" && h2 == "2" && h3 == "3" && h4 == "4" {
		h1, h2, h3, h4 = uniqueMagic()
	}
	return &Config{
		Server: Server{
			PrivateKey: priv, PublicKey: pub,
			Address: m.ipam.ServerIP(),
			Jc:      o.Jc, Jmin: o.Jmin, Jmax: o.Jmax,
			S1: o.S1, S2: o.S2,
			H1: h1, H2: h2, H3: h3, H4: h4,
		},
		Clients: map[string]*Client{},
	}, nil
}

func (m *Manager) persistLocked() error {
	conf, err := RenderServer(m.cur, m.cfg.WGPort)
	if err != nil {
		return err
	}
	return m.store.Save(m.cur, conf)
}

func (m *Manager) saveAndSyncLocked() error {
	if err := m.persistLocked(); err != nil {
		return err
	}
	return m.runner.SyncConf()
}

type ClientView struct {
	*Client
	LatestHandshakeAt *time.Time `json:"latestHandshakeAt"`
	TransferRx        uint64     `json:"transferRx"`
	TransferTx        uint64     `json:"transferTx"`
	PersistentKA      string     `json:"persistentKeepalive"`
}

func (m *Manager) ListClients() ([]ClientView, error) {
	m.mu.Lock()
	out := make([]ClientView, 0, len(m.cur.Clients))
	for _, c := range m.cur.Clients {
		out = append(out, ClientView{Client: c})
	}
	m.mu.Unlock()

	status, err := ShowDump(m.cfg.AWGBin, m.cfg.Interface)
	if err == nil {
		for i := range out {
			if s, ok := status[out[i].PublicKey]; ok {
				out[i].LatestHandshakeAt = s.LatestHandshake
				out[i].TransferRx = s.RxBytes
				out[i].TransferTx = s.TxBytes
				out[i].PersistentKA = s.Keepalive
			}
		}
	}
	return out, nil
}

func (m *Manager) CreateClient(name string) (*Client, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	priv, err := m.keys.GenPrivate()
	if err != nil {
		return nil, err
	}
	pub, err := m.keys.Public(priv)
	if err != nil {
		return nil, err
	}
	psk, err := m.keys.GenPSK()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	used := map[string]struct{}{m.cur.Server.Address: {}}
	for _, c := range m.cur.Clients {
		used[c.Address] = struct{}{}
	}
	addr, err := m.ipam.Next(used)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	c := &Client{
		ID: uuid.NewString(), Name: name, Address: addr,
		PrivateKey: priv, PublicKey: pub, PreSharedKey: psk,
		Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	m.cur.Clients[c.ID] = c
	if err := m.saveAndSyncLocked(); err != nil {
		delete(m.cur.Clients, c.ID)
		return nil, err
	}
	return c, nil
}

func (m *Manager) get(id string) (*Client, error) {
	c, ok := m.cur.Clients[id]
	if !ok {
		return nil, errNotFound
	}
	return c, nil
}

var errNotFound = errors.New("client not found")

func IsNotFound(err error) bool { return errors.Is(err, errNotFound) }

func (m *Manager) DeleteClient(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.cur.Clients[id]; !ok {
		return errNotFound
	}
	delete(m.cur.Clients, id)
	return m.saveAndSyncLocked()
}

func (m *Manager) SetEnabled(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.get(id)
	if err != nil {
		return err
	}
	c.Enabled = enabled
	c.UpdatedAt = time.Now().UTC()
	return m.saveAndSyncLocked()
}

func (m *Manager) Rename(id, name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.get(id)
	if err != nil {
		return err
	}
	c.Name = name
	c.UpdatedAt = time.Now().UTC()
	return m.saveAndSyncLocked()
}

func (m *Manager) SetAddress(id, addr string) error {
	if !m.ipam.Valid(addr) {
		return fmt.Errorf("invalid address: %s", addr)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.get(id)
	if err != nil {
		return err
	}
	for _, other := range m.cur.Clients {
		if other.ID != id && other.Address == addr {
			return fmt.Errorf("address %s already in use", addr)
		}
	}
	c.Address = addr
	c.UpdatedAt = time.Now().UTC()
	return m.saveAndSyncLocked()
}

func (m *Manager) renderArgs(c *Client) ClientRenderArgs {
	return ClientRenderArgs{
		Server:     &m.cur.Server,
		Client:     c,
		DNS:        m.cfg.DNS,
		MTU:        m.cfg.MTU,
		AllowedIPs: m.cfg.AllowedIPs,
		Endpoint:   fmt.Sprintf("%s:%d", m.cfg.WGHost, m.cfg.WGPort),
		Keepalive:  m.cfg.PersistentKA,
	}
}

func (m *Manager) ClientConfig(id string) (*Client, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.get(id)
	if err != nil {
		return nil, nil, err
	}
	out, err := RenderClient(m.renderArgs(c))
	return c, out, err
}

func (m *Manager) ClientAmneziaVPN(id, description string) (*Client, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.get(id)
	if err != nil {
		return nil, "", err
	}
	out, err := RenderAmneziaVPN(m.renderArgs(c), description)
	return c, out, err
}
