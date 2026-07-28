package awg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mrcook1e/amneziawg-panel/internal/config"
)

type profileState struct {
	profile *Profile
	runner  Runner
}

type Manager struct {
	cfg      config.Config
	store    *Store
	keys     Keys
	ipam     *IPAM
	portIPAM *PortIPAM

	mu          sync.Mutex
	profiles    map[string]*profileState
	clients     map[string]*Client
	subscribers map[string]*Subscriber

	emit func(kind, id string, payload any)
}

func (m *Manager) SetEventSink(fn func(kind, id string, payload any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emit = fn
}

func (m *Manager) Config() config.Config {
	return m.cfg
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
	pipam, err := NewPortIPAM(cfg.PortRangeStart, cfg.PortRangeEnd)
	if err != nil {
		return nil, err
	}
	return &Manager{
		cfg:         cfg,
		store:       NewStore(cfg.WGPath),
		keys:        Keys{Bin: cfg.AWGBin},
		ipam:        ipam,
		portIPAM:    pipam,
		profiles:    map[string]*profileState{},
		clients:     map[string]*Client{},
		subscribers: map[string]*Subscriber{},
	}, nil
}

func (m *Manager) Start() error {
	slog.Info("AWG manager starting", slog.String("component", "awg"))

	type bootIface struct {
		iface  string
		runner Runner
	}
	var (
		boot         []bootIface
		profileCount int
	)

	// Phase 1: load state and render conf files under the manager lock.
	if err := func() error {
		m.mu.Lock()
		defer m.mu.Unlock()

		if m.cfg.WGHost == "" {
			return errors.New("WG_HOST is not set")
		}

		c, err := m.store.Load()
		if err != nil {
			slog.Error("AWG state load failed", slog.String("component", "awg"), slog.String("operation", "load_state"), slog.Any("error", err))
			return err
		}
		if c == nil {
			// Fresh install: no auto-bootstrap. Admin creates subscribers and
			// hands out their /cabinet/<token> URLs; subscribers add devices.
			c = &Config{
				SchemaVersion: SchemaVersion,
				Subscribers:   map[string]*Subscriber{},
				Profiles:      map[string]*Profile{},
				Clients:       map[string]*Client{},
			}
		}
		if err := m.validatePersistedMappings(c); err != nil {
			slog.Error("AWG persisted state validation failed", slog.String("component", "awg"), slog.String("operation", "validate_state"), slog.Any("error", err))
			return err
		}
		m.hydrate(c)

		boot = make([]bootIface, 0, len(m.profiles))
		for _, ps := range m.profiles {
			if err := m.persistProfileLocked(ps); err != nil {
				return err
			}
			boot = append(boot, bootIface{iface: ps.profile.Iface, runner: ps.runner})
		}
		if err := m.saveStateLocked("save_start_state"); err != nil {
			return err
		}
		profileCount = len(m.profiles)
		return nil
	}(); err != nil {
		return err
	}

	// Phase 2: bring interfaces up WITHOUT Manager.mu. awg-quick spawns
	// long-lived amneziawg-go; holding the lock here freezes every API that
	// lists subscribers/clients/profiles.
	for _, b := range boot {
		if err := b.runner.Down(); err != nil {
			slog.Warn("AWG interface cleanup failed", slog.String("component", "awg"), slog.String("operation", "start_cleanup"), slog.String("interface", b.iface), slog.Any("error", err))
		}
		if err := b.runner.Up(); err != nil {
			return fmt.Errorf("awg-quick up %s: %w", b.iface, err)
		}
		// Fresh Up already applied conf; SyncConf is belt-and-suspenders and
		// can race the userspace UAPI socket. Retry briefly, then fail.
		var syncErr error
		for attempt := 1; attempt <= 3; attempt++ {
			syncErr = b.runner.SyncConf()
			if syncErr == nil {
				break
			}
			slog.Warn("AWG post-up sync retrying",
				slog.String("component", "awg"),
				slog.String("operation", "start_sync"),
				slog.String("interface", b.iface),
				slog.Int("attempt", attempt),
				slog.Any("error", syncErr),
			)
			time.Sleep(time.Duration(150*attempt) * time.Millisecond)
		}
		if syncErr != nil {
			return syncErr
		}
	}
	slog.Info("AWG manager started", slog.String("component", "awg"), slog.Int("profile_count", profileCount))
	return nil
}

func (m *Manager) Shutdown() error {
	slog.Info("AWG manager stopping", slog.String("component", "awg"))
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ps := range m.profiles {
		if err := ps.runner.Down(); err != nil {
			slog.Warn("AWG shutdown failed", slog.String("component", "awg"), slog.String("operation", "shutdown"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
		}
	}
	slog.Debug("AWG manager stopped", slog.String("component", "awg"))
	return nil
}

func (m *Manager) StateDir() string { return m.cfg.WGPath }

func (m *Manager) IfaceNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.profiles))
	for _, ps := range m.profiles {
		out = append(out, ps.profile.Iface)
	}
	sort.Strings(out)
	return out
}

func (m *Manager) Reload() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.store.Load()
	if err != nil {
		slog.Error("AWG state reload failed", slog.String("component", "awg"), slog.String("operation", "reload_state"), slog.Any("error", err))
		return err
	}
	if c == nil {
		return errors.New("state file missing after reload")
	}
	for _, ps := range m.profiles {
		if err := ps.runner.Down(); err != nil {
			slog.Warn("AWG reload shutdown failed", slog.String("component", "awg"), slog.String("operation", "reload_shutdown"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
		}
	}
	m.profiles = map[string]*profileState{}
	m.clients = map[string]*Client{}
	m.hydrate(c)
	for _, ps := range m.profiles {
		if err := m.persistProfileLocked(ps); err != nil {
			return err
		}
		if err := ps.runner.Down(); err != nil {
			slog.Warn("AWG interface cleanup failed", slog.String("component", "awg"), slog.String("operation", "reload_cleanup"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
		}
		if err := ps.runner.Up(); err != nil {
			return fmt.Errorf("awg-quick up %s: %w", ps.profile.Iface, err)
		}
		if err := ps.runner.SyncConf(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) hydrate(c *Config) {
	for id, p := range c.Profiles {
		m.profiles[id] = &profileState{
			profile: p,
			runner:  Runner{AWGBin: m.cfg.AWGBin, AWGQuickBin: m.cfg.AWGQuickBin, Iface: p.Iface},
		}
	}
	for id, cl := range c.Clients {
		m.clients[id] = cl
	}
	for id, s := range c.Subscribers {
		m.subscribers[id] = s
	}
}

func (m *Manager) dumpStateLocked() *Config {
	profiles := make(map[string]*Profile, len(m.profiles))
	for id, ps := range m.profiles {
		profiles[id] = ps.profile
	}
	clients := make(map[string]*Client, len(m.clients))
	for id, c := range m.clients {
		clients[id] = c
	}
	subs := make(map[string]*Subscriber, len(m.subscribers))
	for id, s := range m.subscribers {
		subs[id] = s
	}
	return &Config{SchemaVersion: SchemaVersion, Subscribers: subs, Profiles: profiles, Clients: clients}
}

// subnetPatternForPort returns the per-profile subnet pattern by replacing the
// third octet of the global pattern with the interface index (port - rangeStart).
// awg0 (port=51820) → "10.8.0.x", awg1 (port=51821) → "10.8.1.x", etc.
// This gives each awgN interface its own /24, eliminating overlapping routes.
func (m *Manager) subnetPatternForPort(port int) string {
	idx := port - m.portIPAM.rangeStart
	parts := strings.SplitN(m.cfg.Subnet, ".", 4) // ["10", "8", "0", "x"]
	if len(parts) == 4 {
		parts[2] = strconv.Itoa(idx)
		return strings.Join(parts, ".")
	}
	return m.cfg.Subnet // fallback: malformed pattern, shouldn't happen
}

func (m *Manager) subnetCIDRForProfile(p *Profile) string {
	return strings.Replace(m.subnetPatternForPort(p.Port), "x", "0", 1) + "/24"
}

// ipamForPort returns an IPAM scoped to the per-profile subnet.
func (m *Manager) ipamForPort(port int) *IPAM {
	ipam, _ := NewIPAM(m.subnetPatternForPort(port))
	return ipam
}

var profileIDRe = regexp.MustCompile(`^[a-z0-9-]{2,32}$`)

func (m *Manager) usedPortsLocked() map[int]struct{} {
	used := map[int]struct{}{}
	for _, ps := range m.profiles {
		used[ps.profile.Port] = struct{}{}
	}
	return used
}

// newProfile builds a fresh Profile from a parsed obfuscation spec. Caller
// (CreateProfile) is responsible for inserting into m.profiles and persisting.
func (m *Manager) newProfile(id, name, description string, spec ObfuscationSpec) (*Profile, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		id = m.nextProfileIDLocked()
	}
	if !profileIDRe.MatchString(id) {
		return nil, fmt.Errorf("profile id must match [a-z0-9-]{2,32}: %q", id)
	}
	if _, exists := m.profiles[id]; exists {
		return nil, fmt.Errorf("profile %q already exists", id)
	}

	port, err := m.portIPAM.Next(m.usedPortsLocked())
	if err != nil {
		return nil, err
	}
	iface := m.portIPAM.IfaceFor(port)

	priv, err := m.keys.GenPrivate()
	if err != nil {
		return nil, err
	}
	pub, err := m.keys.Public(priv)
	if err != nil {
		return nil, err
	}

	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = id
	}

	p := &Profile{
		ID:          id,
		Name:        displayName,
		Description: description,
		Iface:       iface,
		Port:        port,
		PrivateKey:  priv,
		PublicKey:   pub,
		Address:     m.ipamForPort(port).ServerIP(),
		CreatedAt:   time.Now().UTC(),
	}
	applySpec(p, spec)
	return p, nil
}

// applySpec copies every obfuscation field from spec into the profile.
// Centralised so create and patch stay in sync.
func applySpec(p *Profile, s ObfuscationSpec) {
	p.Jc, p.Jmin, p.Jmax = s.Jc, s.Jmin, s.Jmax
	p.S1, p.S2, p.S3, p.S4 = s.S1, s.S2, s.S3, s.S4
	p.H1, p.H2, p.H3, p.H4 = s.H1, s.H2, s.H3, s.H4
	p.I1, p.I2, p.I3, p.I4, p.I5 = s.I1, s.I2, s.I3, s.I4, s.I5
	p.J1, p.J2, p.J3 = s.J1, s.J2, s.J3
	p.Itime = s.Itime
}

func (m *Manager) nextProfileIDLocked() string {
	for n := 1; n < 1000; n++ {
		id := fmt.Sprintf("profile-%d", n)
		if _, exists := m.profiles[id]; !exists {
			return id
		}
	}
	return "profile-" + uuid.NewString()[:8]
}

func (m *Manager) peersForLocked(profileID string) []*Client {
	peers := []*Client{}
	for _, c := range m.clients {
		if c.ProfileID == profileID && c.Enabled {
			peers = append(peers, c)
		}
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].Name < peers[j].Name })
	return peers
}

func (m *Manager) persistProfileLocked(ps *profileState) error {
	conf, err := RenderProfile(ProfileRenderArgs{
		Profile:    ps.profile,
		Peers:      m.peersForLocked(ps.profile.ID),
		SubnetCIDR: m.subnetCIDRForProfile(ps.profile),
		Egress:     m.cfg.EgressIface,
	})
	if err != nil {
		slog.Error("AWG profile render failed", slog.String("component", "awg"), slog.String("operation", "render_profile"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
		return err
	}
	if err := m.store.SaveProfileConf(ps.profile.Iface, conf); err != nil {
		slog.Error("AWG profile save failed", slog.String("component", "awg"), slog.String("operation", "save_profile"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
		return err
	}
	return nil
}

func (m *Manager) persistAllLocked() error {
	if err := m.saveStateLocked("save_all_state"); err != nil {
		return err
	}
	for _, ps := range m.profiles {
		if err := m.persistProfileLocked(ps); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) saveStateLocked(operation string) error {
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		slog.Error("AWG state save failed", slog.String("component", "awg"), slog.String("operation", operation), slog.Any("error", err))
		return err
	}
	return nil
}

func (m *Manager) syncProfileLocked(ps *profileState) error {
	if err := m.persistProfileLocked(ps); err != nil {
		return err
	}
	return ps.runner.SyncConf()
}

// ---- devices ----------------------------------------------------------------

// Direct device creation (without a subscriber+CPS-snippet onboarding flow)
// is gone — see AddDevice in subscriber.go. Only ImportClient remains, as the
// admin's escape hatch for re-attaching a peer with an existing keypair.

type ImportArgs struct {
	Name         string
	SubscriberID string
	ProfileID    string
	PublicKey    string
	PrivateKey   string
	PreSharedKey string
	Address      string
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

	subID := strings.TrimSpace(in.SubscriberID)
	if subID == "" {
		return nil, errors.New("subscriberId is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.subscribers[subID]; !ok {
		return nil, errSubscriberNotFound
	}

	profileID := strings.TrimSpace(in.ProfileID)
	if profileID == "" {
		var chosen *Profile
		for _, ps := range m.profiles {
			if chosen == nil || ps.profile.Port < chosen.Port {
				chosen = ps.profile
			}
		}
		if chosen == nil {
			return nil, errors.New("no profiles configured")
		}
		profileID = chosen.ID
	}
	ps, ok := m.profiles[profileID]
	if !ok {
		return nil, errProfileNotFound
	}

	for _, c := range m.clients {
		if c.PublicKey == pub {
			return nil, fmt.Errorf("device with this publicKey already exists: %s", c.Name)
		}
	}

	used := map[string]struct{}{}
	for _, p := range m.profiles {
		used[p.profile.Address] = struct{}{}
	}
	for _, c := range m.clients {
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
		ID: uuid.NewString(), SubscriberID: subID, ProfileID: profileID,
		Name: name, Address: addr,
		PrivateKey:   strings.TrimSpace(in.PrivateKey),
		PublicKey:    pub,
		PreSharedKey: strings.TrimSpace(in.PreSharedKey),
		Notes:        in.Notes,
		Enabled:      true, CreatedAt: now, UpdatedAt: now,
	}
	m.clients[c.ID] = c
	if err := m.saveStateLocked("save_import_state"); err != nil {
		delete(m.clients, c.ID)
		return nil, err
	}
	if err := m.syncProfileLocked(ps); err != nil {
		delete(m.clients, c.ID)
		return nil, err
	}
	m.fire("device.created", c.ID, map[string]string{"name": c.Name, "address": c.Address, "imported": "true", "subscriberId": subID})
	return c, nil
}

type ClientView struct {
	*Client
	SubscriberName    string     `json:"subscriberName,omitempty"`
	LatestHandshakeAt *time.Time `json:"latestHandshakeAt"`
	TransferRx        uint64     `json:"transferRx"`
	TransferTx        uint64     `json:"transferTx"`
	PersistentKA      string     `json:"persistentKeepalive"`
}

func (m *Manager) ListClients() ([]ClientView, error) {
	m.mu.Lock()
	out := make([]ClientView, 0, len(m.clients))
	for _, c := range m.clients {
		client := *c
		v := ClientView{Client: &client}
		if s, ok := m.subscribers[c.SubscriberID]; ok {
			v.SubscriberName = s.Name
		}
		out = append(out, v)
	}
	m.mu.Unlock()

	status := mergedDump(m.cfg.AWGBin)
	for i := range out {
		if s, ok := status[out[i].PublicKey]; ok {
			out[i].TransferRx = s.RxBytes
			out[i].TransferTx = s.TxBytes
			out[i].PersistentKA = s.Keepalive
			if s.LatestHandshake != nil {
				out[i].LatestHandshakeAt = s.LatestHandshake
			} else {
				out[i].LatestHandshakeAt = out[i].Client.LastHandshakeAt
			}
		} else {
			out[i].LatestHandshakeAt = out[i].Client.LastHandshakeAt
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *Manager) Snapshot() map[string]Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]Client, len(m.clients))
	for _, c := range m.clients {
		out[c.PublicKey] = *c
	}
	return out
}

// ProfileView is the public projection of a Profile returned by /api/profiles/.
// Fields match the TypeScript ProfileInfo interface in web/src/types.ts.
type ProfileView struct {
	Profile
	Endpoint    string `json:"endpoint"`
	ClientCount int    `json:"clientCount"`
	HasMimicry  bool   `json:"hasMimicry"`
}

// ListProfiles returns all profiles enriched with client count and endpoint.
func (m *Manager) ListProfiles() []ProfileView {
	m.mu.Lock()
	defer m.mu.Unlock()
	clientCounts := map[string]int{}
	for _, c := range m.clients {
		clientCounts[c.ProfileID]++
	}
	out := make([]ProfileView, 0, len(m.profiles))
	for _, ps := range m.profiles {
		p := ps.profile
		endpoint := m.cfg.WGHost
		if endpoint != "" {
			endpoint = fmt.Sprintf("%s:%d", endpoint, p.Port)
		}
		out = append(out, ProfileView{
			Profile:     *p,
			Endpoint:    endpoint,
			ClientCount: clientCounts[p.ID],
			HasMimicry:  p.I1 != "" || p.I2 != "" || p.I3 != "" || p.I4 != "" || p.I5 != "",
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

type TrafficUpdate struct {
	ClientID  string
	RxDelta   uint64
	TxDelta   uint64
	Handshake *time.Time
}

// ApplyTrafficBatch persists all collector updates with one state-file write.
func (m *Manager) ApplyTrafficBatch(updates []TrafficUpdate) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dirty := false
	for _, update := range updates {
		if update.RxDelta == 0 && update.TxDelta == 0 && update.Handshake == nil {
			continue
		}
		c, ok := m.clients[update.ClientID]
		if !ok {
			continue
		}
		c.TotalRx += update.RxDelta
		c.TotalTx += update.TxDelta
		if update.Handshake != nil && (c.LastHandshakeAt == nil || update.Handshake.After(*c.LastHandshakeAt)) {
			c.LastHandshakeAt = update.Handshake
		}
		dirty = true
	}
	if dirty {
		if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
			slog.Error("AWG traffic state save failed", slog.String("component", "awg"), slog.String("operation", "save_traffic_state"), slog.Any("error", err))
		}
	}
}

type ClientPatch struct {
	Notes              *string    `json:"notes"`
	ExpiresAt          *time.Time `json:"expiresAt"`
	ClearExpiresAt     bool       `json:"clearExpiresAt"`
	DNSOverride        *string    `json:"dnsOverride"`
	AllowedIPsOverride *string    `json:"allowedIPsOverride"`
	MTUOverride        *int       `json:"mtuOverride"`
	// ItimeOverride: nil = no change. To set, send a pointer to the desired
	// value. To clear, set ClearItimeOverride = true.
	ItimeOverride      *int `json:"itimeOverride"`
	ClearItimeOverride bool `json:"clearItimeOverride"`
}

func (m *Manager) PatchClient(id string, p ClientPatch) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.clients[id]
	if !ok {
		return nil, errNotFound
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
	if p.ClearItimeOverride {
		c.ItimeOverride = nil
	} else if p.ItimeOverride != nil {
		v := *p.ItimeOverride
		c.ItimeOverride = &v
	}
	c.UpdatedAt = time.Now().UTC()
	if err := m.saveStateLocked("save_patch_state"); err != nil {
		return nil, err
	}
	m.fire("client.patched", id, map[string]string{"name": c.Name})
	cp := *c
	return &cp, nil
}

func (m *Manager) DisableExpired(now time.Time) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var flipped []string
	dirtyProfiles := map[string]struct{}{}
	for _, c := range m.clients {
		if c.Enabled && c.ExpiresAt != nil && c.ExpiresAt.Before(now) {
			c.Enabled = false
			c.UpdatedAt = now.UTC()
			flipped = append(flipped, c.ID)
			dirtyProfiles[c.ProfileID] = struct{}{}
		}
	}
	if len(flipped) > 0 {
		if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
			slog.Error("AWG expiry state save failed", slog.String("component", "awg"), slog.String("operation", "save_expiry_state"), slog.Any("error", err))
		}
		for pid := range dirtyProfiles {
			if ps, ok := m.profiles[pid]; ok {
				if err := m.syncProfileLocked(ps); err != nil {
					slog.Error("AWG expiry configuration sync failed", slog.String("component", "awg"), slog.String("operation", "sync_expiry_profile"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
				}
			}
		}
		for _, id := range flipped {
			m.fire("client.expired", id, nil)
		}
	}
	return flipped
}

// Device deletion lives in subscriber.go (DeleteDevice). Old DeleteClient that
// removed the peer record but kept the orphan interface alive is gone — every
// device owns its profile 1:1, so deletion cascades.

func (m *Manager) SetEnabled(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.clients[id]
	if !ok {
		return errNotFound
	}
	c.Enabled = enabled
	c.UpdatedAt = time.Now().UTC()
	if err := m.saveStateLocked("save_enabled_state"); err != nil {
		return err
	}
	if ps, ok := m.profiles[c.ProfileID]; ok {
		if err := m.syncProfileLocked(ps); err != nil {
			return err
		}
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
	c, ok := m.clients[id]
	if !ok {
		return errNotFound
	}
	prev := c.Name
	c.Name = name
	c.UpdatedAt = time.Now().UTC()
	if err := m.saveStateLocked("save_rename_state"); err != nil {
		return err
	}
	if ps, ok := m.profiles[c.ProfileID]; ok {
		if err := m.syncProfileLocked(ps); err != nil {
			return err
		}
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
	c, ok := m.clients[id]
	if !ok {
		return errNotFound
	}
	for _, other := range m.clients {
		if other.ID != id && other.Address == addr {
			return fmt.Errorf("address %s already in use", addr)
		}
	}
	for _, ps := range m.profiles {
		if ps.profile.Address == addr {
			return fmt.Errorf("address %s already in use by profile %s", addr, ps.profile.ID)
		}
	}
	c.Address = addr
	c.UpdatedAt = time.Now().UTC()
	if err := m.saveStateLocked("save_address_state"); err != nil {
		return err
	}
	if ps, ok := m.profiles[c.ProfileID]; ok {
		return m.syncProfileLocked(ps)
	}
	return nil
}

// MoveClient was the cross-profile relocation primitive — irrelevant in the
// 1 device = 1 profile model. Removed.

func (m *Manager) renderArgs(profile *Profile, c *Client) ClientRenderArgs {
	return ClientRenderArgs{
		Profile:    profile,
		Client:     c,
		DNS:        m.cfg.DNS,
		MTU:        m.cfg.MTU,
		AllowedIPs: m.cfg.AllowedIPs,
		Endpoint:   fmt.Sprintf("%s:%d", m.cfg.WGHost, profile.Port),
		Keepalive:  m.cfg.PersistentKA,
		// Itime is resolved inside RenderClient from profile + per-client override.
	}
}

func (m *Manager) ClientConfig(id string) (*Client, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.clients[id]
	if !ok {
		return nil, nil, errNotFound
	}
	ps, ok := m.profiles[c.ProfileID]
	if !ok {
		return nil, nil, errProfileNotFound
	}
	out, err := RenderClient(m.renderArgs(ps.profile, c))
	return c, out, err
}

// ListPayerSubscriberIDs returns a deterministic list of subscriber IDs that are payers.
func (m *Manager) ListPayerSubscriberIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	for _, s := range m.subscribers {
		role := s.BillingRole
		if role == "" {
			role = BillingRoleTrusted
		}
		if role == BillingRolePayer {
			ids = append(ids, s.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// DeviceIDsBySubscriber returns the device (client) IDs owned by a subscriber.
// Used by billing to aggregate per-subscriber traffic from SQLite.
func (m *Manager) DeviceIDsBySubscriber(subscriberID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0)
	for _, c := range m.clients {
		if c.SubscriberID == subscriberID {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// SuspendSubscriberClients suspends currently enabled clients of a subscriber by setting Enabled=false and BillingSuspended=true.
func (m *Manager) SuspendSubscriberClients(subscriberID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub, ok := m.subscribers[subscriberID]
	if !ok {
		return errSubscriberNotFound
	}
	if sub.BillingRole != BillingRolePayer {
		return nil
	}

	dirtyProfiles := map[string]struct{}{}
	anyChanged := false
	for _, c := range m.clients {
		if c.SubscriberID == subscriberID {
			if c.Enabled {
				c.Enabled = false
				c.BillingSuspended = true
				c.UpdatedAt = time.Now().UTC()
				dirtyProfiles[c.ProfileID] = struct{}{}
				anyChanged = true
			}
		}
	}

	if anyChanged {
		if err := m.saveStateLocked("save_billing_suspension_state"); err != nil {
			return err
		}
		for pid := range dirtyProfiles {
			if ps, ok := m.profiles[pid]; ok {
				if err := m.syncProfileLocked(ps); err != nil {
					slog.Error("AWG suspended profile sync failed", slog.String("component", "awg"), slog.String("operation", "sync_suspended_profile"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
				}
			}
		}
	}
	return nil
}

// ResumeSubscriberClients resumes ONLY BillingSuspended clients by clearing BillingSuspended and enabling them.
func (m *Manager) ResumeSubscriberClients(subscriberID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dirtyProfiles := map[string]struct{}{}
	anyChanged := false
	for _, c := range m.clients {
		if c.SubscriberID == subscriberID {
			if c.BillingSuspended {
				c.Enabled = true
				c.BillingSuspended = false
				c.UpdatedAt = time.Now().UTC()
				dirtyProfiles[c.ProfileID] = struct{}{}
				anyChanged = true
			}
		}
	}

	if anyChanged {
		if err := m.saveStateLocked("save_billing_resume_state"); err != nil {
			return err
		}
		for pid := range dirtyProfiles {
			if ps, ok := m.profiles[pid]; ok {
				if err := m.syncProfileLocked(ps); err != nil {
					slog.Error("AWG resumed profile sync failed", slog.String("component", "awg"), slog.String("operation", "sync_resumed_profile"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
				}
			}
		}
	}
	return nil
}

// ResetClients wipes every client from every profile.
func (m *Manager) ResetClients() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.clients)
	m.clients = map[string]*Client{}
	if err := m.saveStateLocked("save_reset_state"); err != nil {
		return err
	}
	for _, ps := range m.profiles {
		if err := m.syncProfileLocked(ps); err != nil {
			return err
		}
	}
	m.fire("server.reset_clients", "", map[string]int{"removed": n})
	return nil
}

// FactoryReset drops every profile and every client, leaving the server in
// the same state as a fresh install: no profiles, no clients, no interfaces
// running. The admin must create the first profile via the API.
func (m *Manager) FactoryReset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ps := range m.profiles {
		if err := ps.runner.Down(); err != nil {
			slog.Warn("AWG reset shutdown failed", slog.String("component", "awg"), slog.String("operation", "factory_reset_shutdown"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
		}
		if err := m.store.RemoveProfileConf(ps.profile.Iface); err != nil {
			slog.Warn("AWG profile removal failed", slog.String("component", "awg"), slog.String("operation", "remove_profile_config"), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
		}
	}
	m.profiles = map[string]*profileState{}
	m.clients = map[string]*Client{}
	m.subscribers = map[string]*Subscriber{}

	if err := m.saveStateLocked("save_factory_reset_state"); err != nil {
		return err
	}
	m.fire("server.factory_reset", "", nil)
	return nil
}

var (
	errNotFound        = errors.New("client not found")
	errProfileNotFound = errors.New("profile not found")
)

func IsNotFound(err error) bool {
	return errors.Is(err, errNotFound) || errors.Is(err, errProfileNotFound)
}

// mergedDump returns all live peers through one UAPI process.
func mergedDump(bin string) map[string]PeerStatus {
	out, err := ShowAllDump(context.Background(), bin)
	if err != nil {
		slog.Warn("AWG interface read failed", slog.String("component", "awg"), slog.String("operation", "show_all_dump"), slog.Any("error", err))
		return map[string]PeerStatus{}
	}
	return out
}
