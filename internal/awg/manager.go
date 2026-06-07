package awg

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
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
		cfg:      cfg,
		store:    NewStore(cfg.WGPath),
		keys:     Keys{Bin: cfg.AWGBin},
		ipam:     ipam,
		portIPAM: pipam,
		profiles:    map[string]*profileState{},
		clients:     map[string]*Client{},
		subscribers: map[string]*Subscriber{},
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
		// Fresh install: no auto-bootstrap. Admin creates subscribers and
		// hands out their /cabinet/<token> URLs; subscribers add devices.
		c = &Config{
			SchemaVersion: SchemaVersion,
			Subscribers:   map[string]*Subscriber{},
			Profiles:      map[string]*Profile{},
			Clients:       map[string]*Client{},
		}
	}
	m.hydrate(c)

	for _, ps := range m.profiles {
		if err := m.persistProfileLocked(ps); err != nil {
			return err
		}
		_ = ps.runner.Down()
		if err := ps.runner.Up(); err != nil {
			return fmt.Errorf("awg-quick up %s: %w", ps.profile.Iface, err)
		}
		if err := ps.runner.SyncConf(); err != nil {
			return err
		}
	}
	return m.store.SaveState(m.dumpStateLocked())
}

func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ps := range m.profiles {
		_ = ps.runner.Down()
	}
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
		return err
	}
	if c == nil {
		return errors.New("state file missing after reload")
	}
	for _, ps := range m.profiles {
		_ = ps.runner.Down()
	}
	m.profiles = map[string]*profileState{}
	m.clients = map[string]*Client{}
	m.hydrate(c)
	for _, ps := range m.profiles {
		if err := m.persistProfileLocked(ps); err != nil {
			return err
		}
		_ = ps.runner.Down()
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

func (m *Manager) subnetCIDR() string {
	return strings.Replace(m.cfg.Subnet, "x", "0", 1) + "/24"
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
		Address:     m.ipam.ServerIP(),
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
		SubnetCIDR: m.subnetCIDR(),
		Egress:     m.cfg.EgressIface,
	})
	if err != nil {
		return err
	}
	return m.store.SaveProfileConf(ps.profile.Iface, conf)
}

func (m *Manager) persistAllLocked() error {
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		return err
	}
	for _, ps := range m.profiles {
		if err := m.persistProfileLocked(ps); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) syncProfileLocked(ps *profileState) error {
	if err := m.persistProfileLocked(ps); err != nil {
		return err
	}
	return ps.runner.SyncConf()
}

// ---- profiles ---------------------------------------------------------------

type ProfileSpec struct {
	ID, Name, Description string
	Obf                   ObfuscationSpec
}

type ProfileView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Iface       string `json:"iface"`
	Port        int    `json:"port"`
	PublicKey   string `json:"publicKey"`
	Address     string `json:"address"`
	Endpoint    string `json:"endpoint"`

	Jc    int    `json:"jc"`
	Jmin  int    `json:"jmin"`
	Jmax  int    `json:"jmax"`
	S1    int    `json:"s1"`
	S2    int    `json:"s2"`
	S3    int    `json:"s3"`
	S4    int    `json:"s4"`
	H1    string `json:"h1"`
	H2    string `json:"h2"`
	H3    string `json:"h3"`
	H4    string `json:"h4"`
	I1    string `json:"i1,omitempty"`
	I2    string `json:"i2,omitempty"`
	I3    string `json:"i3,omitempty"`
	I4    string `json:"i4,omitempty"`
	I5    string `json:"i5,omitempty"`
	J1    string `json:"j1,omitempty"`
	J2    string `json:"j2,omitempty"`
	J3    string `json:"j3,omitempty"`
	Itime int    `json:"itime"`

	ClientCount int  `json:"clientCount"`
	HasMimicry  bool `json:"hasMimicry"`
}

func (m *Manager) viewProfileLocked(p *Profile) ProfileView {
	count := 0
	for _, c := range m.clients {
		if c.ProfileID == p.ID {
			count++
		}
	}
	return ProfileView{
		ID: p.ID, Name: p.Name, Description: p.Description,
		Iface: p.Iface, Port: p.Port, PublicKey: p.PublicKey, Address: p.Address,
		Endpoint: fmt.Sprintf("%s:%d", m.cfg.WGHost, p.Port),
		Jc:       p.Jc, Jmin: p.Jmin, Jmax: p.Jmax,
		S1: p.S1, S2: p.S2, S3: p.S3, S4: p.S4,
		H1: p.H1, H2: p.H2, H3: p.H3, H4: p.H4,
		I1: p.I1, I2: p.I2, I3: p.I3, I4: p.I4, I5: p.I5,
		J1: p.J1, J2: p.J2, J3: p.J3,
		Itime:       p.Itime,
		ClientCount: count,
		HasMimicry:  p.I1 != "" || p.I2 != "" || p.I3 != "" || p.I4 != "" || p.I5 != "",
	}
}

func (m *Manager) ListProfiles() []ProfileView {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ProfileView, 0, len(m.profiles))
	for _, ps := range m.profiles {
		out = append(out, m.viewProfileLocked(ps.profile))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out
}

func (m *Manager) ProfileInfo(id string) (ProfileView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.profiles[id]
	if !ok {
		return ProfileView{}, errProfileNotFound
	}
	return m.viewProfileLocked(ps.profile), nil
}

// DefaultProfileID returns the profile selected when the UI doesn't ask for a
// specific one. Lowest-port profile wins (deterministic). Empty string if none.
func (m *Manager) DefaultProfileID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var chosen *Profile
	for _, ps := range m.profiles {
		if chosen == nil || ps.profile.Port < chosen.Port {
			chosen = ps.profile
		}
	}
	if chosen == nil {
		return ""
	}
	return chosen.ID
}

func (m *Manager) CreateProfile(spec ProfileSpec) (ProfileView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, err := m.newProfile(spec.ID, spec.Name, spec.Description, spec.Obf)
	if err != nil {
		return ProfileView{}, err
	}

	ps := &profileState{
		profile: p,
		runner:  Runner{AWGBin: m.cfg.AWGBin, AWGQuickBin: m.cfg.AWGQuickBin, Iface: p.Iface},
	}
	m.profiles[p.ID] = ps

	if err := m.persistAllLocked(); err != nil {
		delete(m.profiles, p.ID)
		return ProfileView{}, err
	}
	_ = ps.runner.Down()
	if err := ps.runner.Up(); err != nil {
		delete(m.profiles, p.ID)
		_ = m.persistAllLocked()
		return ProfileView{}, fmt.Errorf("awg-quick up %s: %w", p.Iface, err)
	}
	m.fire("profile.created", p.ID, map[string]any{"name": p.Name, "iface": p.Iface, "port": p.Port})
	return m.viewProfileLocked(p), nil
}

type ProfilePatch struct {
	Name        *string
	Description *string
	// Obf, when non-nil, replaces the whole obfuscation block atomically.
	// Parsed and validated by the caller (handler) before reaching here.
	Obf *ObfuscationSpec
}

func (m *Manager) PatchProfile(id string, p ProfilePatch) (ProfileView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.profiles[id]
	if !ok {
		return ProfileView{}, errProfileNotFound
	}
	pr := ps.profile
	if p.Name != nil {
		pr.Name = strings.TrimSpace(*p.Name)
		if pr.Name == "" {
			pr.Name = pr.ID
		}
	}
	if p.Description != nil {
		pr.Description = *p.Description
	}
	if p.Obf != nil {
		applySpec(pr, *p.Obf)
	}
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		return ProfileView{}, err
	}
	if err := m.syncProfileLocked(ps); err != nil {
		return ProfileView{}, err
	}
	m.fire("profile.patched", pr.ID, map[string]string{"name": pr.Name})
	return m.viewProfileLocked(pr), nil
}

func (m *Manager) DeleteProfile(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.profiles[id]
	if !ok {
		return errProfileNotFound
	}
	for _, c := range m.clients {
		if c.ProfileID == id {
			return errProfileHasClients
		}
	}
	_ = ps.runner.Down()
	_ = m.store.RemoveProfileConf(ps.profile.Iface)
	delete(m.profiles, id)
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		return err
	}
	m.fire("profile.deleted", id, nil)
	return nil
}

func (m *Manager) RestartInterface(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.profiles[profileID]
	if !ok {
		return errProfileNotFound
	}
	if err := m.persistProfileLocked(ps); err != nil {
		return err
	}
	_ = ps.runner.Down()
	if err := ps.runner.Up(); err != nil {
		return err
	}
	m.fire("profile.restart", profileID, nil)
	return nil
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
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
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
		v := ClientView{Client: c}
		if s, ok := m.subscribers[c.SubscriberID]; ok {
			v.SubscriberName = s.Name
		}
		out = append(out, v)
	}
	ifaces := make([]string, 0, len(m.profiles))
	for _, ps := range m.profiles {
		ifaces = append(ifaces, ps.profile.Iface)
	}
	m.mu.Unlock()

	status := mergedDump(m.cfg.AWGBin, ifaces)
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

func (m *Manager) ApplyTraffic(id string, rxDelta, txDelta uint64, handshake *time.Time) {
	if rxDelta == 0 && txDelta == 0 && handshake == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.clients[id]
	if !ok {
		return
	}
	c.TotalRx += rxDelta
	c.TotalTx += txDelta
	if handshake != nil && (c.LastHandshakeAt == nil || handshake.After(*c.LastHandshakeAt)) {
		c.LastHandshakeAt = handshake
	}
	_ = m.store.SaveState(m.dumpStateLocked())
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
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
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
		_ = m.store.SaveState(m.dumpStateLocked())
		for pid := range dirtyProfiles {
			if ps, ok := m.profiles[pid]; ok {
				_ = m.syncProfileLocked(ps)
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
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
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
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
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
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
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

// ResetClients wipes every client from every profile.
func (m *Manager) ResetClients() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := len(m.clients)
	m.clients = map[string]*Client{}
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
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
		_ = ps.runner.Down()
		_ = m.store.RemoveProfileConf(ps.profile.Iface)
	}
	m.profiles = map[string]*profileState{}
	m.clients = map[string]*Client{}
	m.subscribers = map[string]*Subscriber{}

	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		return err
	}
	m.fire("server.factory_reset", "", nil)
	return nil
}

var (
	errNotFound          = errors.New("client not found")
	errProfileNotFound   = errors.New("profile not found")
	errProfileHasClients = errors.New("profile has clients; move or delete them first")
)

func IsNotFound(err error) bool        { return errors.Is(err, errNotFound) || errors.Is(err, errProfileNotFound) }
func IsProfileHasClients(err error) bool { return errors.Is(err, errProfileHasClients) }

// mergedDump runs `awg show <iface> dump` for each interface and returns a
// single map keyed by peer pubkey. Failures on a single interface are silently
// skipped — happens during a profile restart.
func mergedDump(bin string, ifaces []string) map[string]PeerStatus {
	out := map[string]PeerStatus{}
	for _, iface := range ifaces {
		st, err := ShowDump(bin, iface)
		if err != nil {
			continue
		}
		for k, v := range st {
			out[k] = v
		}
	}
	return out
}
