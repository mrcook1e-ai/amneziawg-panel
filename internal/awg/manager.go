package awg

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

	// emit is a hook used to journal lifecycle events without importing the
	// events package directly (would cycle). Wired by main.go.
	emit func(kind, clientID string, payload any)
}

// SetEventSink wires the audit-log emitter. Safe to leave unset (no-op).
func (m *Manager) SetEventSink(fn func(kind, clientID string, payload any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emit = fn
}

func (m *Manager) fire(kind, id string, payload any) {
	if m.emit != nil {
		m.emit(kind, id, payload)
	}
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

// StateDir returns the directory that holds <iface>.json / <iface>.conf.
// Used by backup/restore to know what to bundle.
func (m *Manager) StateDir() string  { return m.cfg.WGPath }
func (m *Manager) IfaceName() string { return m.cfg.Interface }

// Reload re-reads the JSON state from disk (after a restore has dropped a new
// file in place) and bounces the interface so the running peers match.
func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.store.Load()
	if err != nil {
		return err
	}
	if c == nil {
		return errors.New("state file missing after reload")
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

// ImportArgs lets an admin re-attach an existing peer (e.g. they kept the
// privateKey on their phone). PublicKey is required; if PrivateKey is empty
// the panel won't be able to render a downloadable config but the peer still
// shows up on the dashboard and counts toward stats.
type ImportArgs struct {
	Name         string
	PublicKey    string
	PrivateKey   string
	PreSharedKey string
	Address      string // optional — auto-allocated if empty
	Notes        string
}

func (m *Manager) ImportClient(in ImportArgs) (*Client, error) {
	name := strings.TrimSpace(in.Name)
	pub := strings.TrimSpace(in.PublicKey)
	if name == "" {
		return nil, errors.New("name is required")
	}
	if pub == "" {
		return nil, errors.New("publicKey is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.cur.Clients {
		if c.PublicKey == pub {
			return nil, fmt.Errorf("client with this publicKey already exists: %s", c.Name)
		}
	}

	used := map[string]struct{}{m.cur.Server.Address: {}}
	for _, c := range m.cur.Clients {
		used[c.Address] = struct{}{}
	}
	addr := strings.TrimSpace(in.Address)
	if addr == "" {
		var err error
		addr, err = m.ipam.Next(used)
		if err != nil {
			return nil, err
		}
	} else {
		if !m.ipam.Valid(addr) {
			return nil, fmt.Errorf("invalid address: %s", addr)
		}
		if _, taken := used[addr]; taken {
			return nil, fmt.Errorf("address %s already in use", addr)
		}
	}

	now := time.Now().UTC()
	c := &Client{
		ID: uuid.NewString(), Name: name, Address: addr,
		PrivateKey:   strings.TrimSpace(in.PrivateKey),
		PublicKey:    pub,
		PreSharedKey: strings.TrimSpace(in.PreSharedKey),
		Notes:        in.Notes,
		Enabled:      true, CreatedAt: now, UpdatedAt: now,
	}
	m.cur.Clients[c.ID] = c
	if err := m.saveAndSyncLocked(); err != nil {
		delete(m.cur.Clients, c.ID)
		return nil, err
	}
	m.fire("client.created", c.ID, map[string]string{"name": c.Name, "address": c.Address, "imported": "true"})
	return c, nil
}

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

func (m *Manager) subnetCIDR() string {
	return strings.Replace(m.cfg.Subnet, "x", "0", 1) + "/24"
}

func (m *Manager) persistLocked() error {
	conf, err := RenderServer(ServerRenderArgs{
		Config:     m.cur,
		Port:       m.cfg.WGPort,
		SubnetCIDR: m.subnetCIDR(),
		Egress:     m.cfg.EgressIface,
	})
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
	// Session counters from the running interface — reset to zero on
	// awg-quick up. For lifetime totals use Client.TotalRx/TotalTx.
	TransferRx   uint64 `json:"transferRx"`
	TransferTx   uint64 `json:"transferTx"`
	PersistentKA string `json:"persistentKeepalive"`
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
				out[i].TransferRx = s.RxBytes
				out[i].TransferTx = s.TxBytes
				out[i].PersistentKA = s.Keepalive
				// Prefer the freshest handshake — current session if available,
				// otherwise the last-seen value persisted across restarts.
				if s.LatestHandshake != nil {
					out[i].LatestHandshakeAt = s.LatestHandshake
				} else {
					out[i].LatestHandshakeAt = out[i].Client.LastHandshakeAt
				}
			} else {
				out[i].LatestHandshakeAt = out[i].Client.LastHandshakeAt
			}
		}
	}
	return out, nil
}

// Snapshot returns a read-only copy of the current client map keyed by public
// key. Used by the stats collector to reconcile peer status without taking
// the write path.
func (m *Manager) Snapshot() map[string]Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]Client, len(m.cur.Clients))
	for _, c := range m.cur.Clients {
		out[c.PublicKey] = *c
	}
	return out
}

// ApplyTraffic adds the given bucket deltas to a client's lifetime counters
// and updates last-handshake. Called by the stats collector. No-op if the
// client no longer exists.
func (m *Manager) ApplyTraffic(id string, rxDelta, txDelta uint64, handshake *time.Time) {
	if rxDelta == 0 && txDelta == 0 && handshake == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.cur.Clients[id]
	if !ok {
		return
	}
	c.TotalRx += rxDelta
	c.TotalTx += txDelta
	if handshake != nil && (c.LastHandshakeAt == nil || handshake.After(*c.LastHandshakeAt)) {
		c.LastHandshakeAt = handshake
	}
	// Cheap persist — avoid syncconf, this only changed accounting.
	_ = m.persistLocked()
}

// ClientPatch captures user-editable fields. Pointer types let us distinguish
// "leave alone" (nil) from "clear" (pointer to zero value).
type ClientPatch struct {
	Notes              *string    `json:"notes"`
	ExpiresAt          *time.Time `json:"expiresAt"` // null in JSON clears it
	ClearExpiresAt     bool       `json:"clearExpiresAt"`
	DNSOverride        *string    `json:"dnsOverride"`
	AllowedIPsOverride *string    `json:"allowedIPsOverride"`
	MTUOverride        *int       `json:"mtuOverride"`
}

func (m *Manager) PatchClient(id string, p ClientPatch) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.get(id)
	if err != nil {
		return nil, err
	}
	if p.Notes != nil {
		c.Notes = *p.Notes
	}
	if p.ClearExpiresAt {
		c.ExpiresAt = nil
	} else if p.ExpiresAt != nil {
		t := p.ExpiresAt.UTC()
		c.ExpiresAt = &t
	}
	if p.DNSOverride != nil {
		c.DNSOverride = strings.TrimSpace(*p.DNSOverride)
	}
	if p.AllowedIPsOverride != nil {
		c.AllowedIPsOverride = strings.TrimSpace(*p.AllowedIPsOverride)
	}
	if p.MTUOverride != nil {
		c.MTUOverride = *p.MTUOverride
	}
	c.UpdatedAt = time.Now().UTC()
	// Overrides don't affect server.conf — patch is config-rendering metadata.
	// We still persist; syncconf is a no-op (no peer entries changed).
	if err := m.persistLocked(); err != nil {
		return nil, err
	}
	m.fire("client.patched", id, map[string]string{"name": c.Name})
	cp := *c
	return &cp, nil
}

// DisableExpired walks clients and disables any whose ExpiresAt is in the
// past. Returns the IDs that flipped. Idempotent.
func (m *Manager) DisableExpired(now time.Time) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var flipped []string
	for _, c := range m.cur.Clients {
		if c.Enabled && c.ExpiresAt != nil && c.ExpiresAt.Before(now) {
			c.Enabled = false
			c.UpdatedAt = now.UTC()
			flipped = append(flipped, c.ID)
		}
	}
	if len(flipped) > 0 {
		_ = m.saveAndSyncLocked()
		for _, id := range flipped {
			m.fire("client.expired", id, nil)
		}
	}
	return flipped
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
	m.fire("client.created", c.ID, map[string]string{"name": c.Name, "address": c.Address})
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
	c, ok := m.cur.Clients[id]
	if !ok {
		return errNotFound
	}
	name := c.Name
	delete(m.cur.Clients, id)
	if err := m.saveAndSyncLocked(); err != nil {
		return err
	}
	m.fire("client.deleted", id, map[string]string{"name": name})
	return nil
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
	if err := m.saveAndSyncLocked(); err != nil {
		return err
	}
	kind := "client.enabled"
	if !enabled {
		kind = "client.disabled"
	}
	m.fire(kind, id, map[string]string{"name": c.Name})
	return nil
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
	prev := c.Name
	c.Name = name
	c.UpdatedAt = time.Now().UTC()
	if err := m.saveAndSyncLocked(); err != nil {
		return err
	}
	m.fire("client.renamed", id, map[string]string{"from": prev, "to": name})
	return nil
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

type ServerView struct {
	PublicKey     string `json:"publicKey"`
	Address       string `json:"address"`
	Interface     string `json:"interface"`
	Endpoint      string `json:"endpoint"`
	Subnet        string `json:"subnet"`
	Port          int    `json:"port"`
	EgressIface   string `json:"egressIface"`
	DNS           string `json:"dns"`
	MTU           int    `json:"mtu"`
	AllowedIPs    string `json:"allowedIPs"`
	Keepalive     int    `json:"persistentKeepalive"`
	Jc            int    `json:"jc"`
	Jmin          int    `json:"jmin"`
	Jmax          int    `json:"jmax"`
	S1            int    `json:"s1"`
	S2            int    `json:"s2"`
	H1            string `json:"h1"`
	H2            string `json:"h2"`
	H3            string `json:"h3"`
	H4            string `json:"h4"`
	ClientCount   int    `json:"clientCount"`
}

func (m *Manager) ServerInfo() ServerView {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.cur.Server
	return ServerView{
		PublicKey:   s.PublicKey,
		Address:     s.Address,
		Interface:   m.cfg.Interface,
		Endpoint:    fmt.Sprintf("%s:%d", m.cfg.WGHost, m.cfg.WGPort),
		Subnet:      m.subnetCIDR(),
		Port:        m.cfg.WGPort,
		EgressIface: m.cfg.EgressIface,
		DNS:         m.cfg.DNS,
		MTU:         m.cfg.MTU,
		AllowedIPs:  m.cfg.AllowedIPs,
		Keepalive:   m.cfg.PersistentKA,
		Jc:          s.Jc, Jmin: s.Jmin, Jmax: s.Jmax,
		S1: s.S1, S2: s.S2,
		H1: s.H1, H2: s.H2, H3: s.H3, H4: s.H4,
		ClientCount: len(m.cur.Clients),
	}
}

func (m *Manager) RegenerateMagic() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cur.Server.H1, m.cur.Server.H2, m.cur.Server.H3, m.cur.Server.H4 = uniqueMagic()
	if err := m.saveAndSyncLocked(); err != nil {
		return err
	}
	m.fire("server.regen_magic", "", nil)
	return nil
}

// RestartInterface tears the interface down and brings it back up. PostUp/
// PostDown fire as a side effect, so iptables rules are reseated cleanly.
func (m *Manager) RestartInterface() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.persistLocked(); err != nil {
		return err
	}
	_ = m.runner.Down()
	if err := m.runner.Up(); err != nil {
		return err
	}
	m.fire("server.restart", "", nil)
	return nil
}

// FactoryReset возвращает сервер в состояние «свежая установка»: новый
// ключ сервера, новые H1–H4, пустой список клиентов. Интерфейс
// перезапускается чтобы старые соединения отвалились. Метрики и журнал
// событий очищаются вызывающей стороной (это пакет stats, отдельно).
func (m *Manager) FactoryReset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	priv, err := m.keys.GenPrivate()
	if err != nil {
		return err
	}
	pub, err := m.keys.Public(priv)
	if err != nil {
		return err
	}
	h1, h2, h3, h4 := uniqueMagic()

	n := len(m.cur.Clients)
	m.cur.Server.PrivateKey = priv
	m.cur.Server.PublicKey = pub
	m.cur.Server.H1, m.cur.Server.H2, m.cur.Server.H3, m.cur.Server.H4 = h1, h2, h3, h4
	m.cur.Clients = map[string]*Client{}

	if err := m.persistLocked(); err != nil {
		return err
	}
	_ = m.runner.Down()
	if err := m.runner.Up(); err != nil {
		return err
	}
	m.fire("server.factory_reset", "", map[string]int{"removedClients": n})
	return nil
}

func (m *Manager) ResetClients() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.cur.Clients)
	m.cur.Clients = map[string]*Client{}
	if err := m.saveAndSyncLocked(); err != nil {
		return err
	}
	m.fire("server.reset_clients", "", map[string]int{"removed": n})
	return nil
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
