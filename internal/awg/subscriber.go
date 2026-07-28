package awg

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// tokenEqual compares two access tokens in constant time to avoid leaking
// length-prefix matches via response timing. Length mismatch is checked first
// (cheap) since equal-length is the only case where ConstantTimeCompare makes
// sense.
func tokenEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

const (
	BillingRoleOwner   = "owner"
	BillingRoleTrusted = "trusted"
	BillingRolePayer   = "payer"
)

func normalizeBillingRole(role string) (string, error) {
	switch strings.TrimSpace(role) {
	case "", BillingRoleTrusted:
		return BillingRoleTrusted, nil
	case BillingRoleOwner:
		return BillingRoleOwner, nil
	case BillingRolePayer:
		return BillingRolePayer, nil
	default:
		return "", fmt.Errorf("invalid billingRole: %s", role)
	}
}

// Subscriber is the human-level account: one named person who may own many
// VPN devices (each device = one Client + one Profile = one awgN interface,
// because obfuscation params are per-interface).
//
// AccessToken IS the credential to the subscriber's personal cabinet
// (/cabinet/<token>). A 256-bit url-safe random — magic-link auth, same model
// as Google Docs share-links. Regenerable by admin to invalidate the old URL.
type Subscriber struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	AccessToken string    `json:"accessToken"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	BillingRole string    `json:"billingRole"`
}

// SubscriberView is the admin-facing projection with device count and the
// constructed cabinet URL (resolved per-request from Host header).
type SubscriberView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	AccessToken string    `json:"accessToken"`
	URL         string    `json:"url"`
	Notes       string    `json:"notes,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	DeviceCount int       `json:"deviceCount"`
	Devices     []Client  `json:"devices,omitempty"`
	BillingRole string    `json:"billingRole"`
}

// CabinetView is what the subscriber sees in their cabinet. Excludes
// AccessToken and internal IDs that aren't useful.
type CabinetView struct {
	Name    string          `json:"name"`
	Devices []CabinetDevice `json:"devices"`
}

type CabinetDevice struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Address           string     `json:"address"`
	Enabled           bool       `json:"enabled"`
	CreatedAt         time.Time  `json:"createdAt"`
	LatestHandshakeAt *time.Time `json:"latestHandshakeAt,omitempty"`
}

func newAccessToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// CreateSubscriber: admin creates a new account. Returns the fresh record
// including its accessToken (so the admin can copy the cabinet URL).
func (m *Manager) CreateSubscriber(name, notes string, billingRole ...string) (*Subscriber, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	tok, err := newAccessToken()
	if err != nil {
		return nil, err
	}
	role := BillingRoleTrusted
	if len(billingRole) > 0 {
		role, err = normalizeBillingRole(billingRole[0])
		if err != nil {
			return nil, err
		}
	}
	s := &Subscriber{
		ID:          uuid.NewString()[:8],
		Name:        name,
		AccessToken: tok,
		Notes:       strings.TrimSpace(notes),
		CreatedAt:   time.Now().UTC(),
		BillingRole: role,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.subscribers == nil {
		m.subscribers = map[string]*Subscriber{}
	}
	m.subscribers[s.ID] = s
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		delete(m.subscribers, s.ID)
		return nil, err
	}
	m.fire("subscriber.created", s.ID, map[string]string{"name": s.Name})
	return s, nil
}

func (m *Manager) ListSubscribers() []SubscriberView {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]SubscriberView, 0, len(m.subscribers))
	for _, s := range m.subscribers {
		out = append(out, m.viewSubscriberLocked(s, false))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (m *Manager) SubscriberDetail(id string) (SubscriberView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subscribers[id]
	if !ok {
		return SubscriberView{}, errSubscriberNotFound
	}
	return m.viewSubscriberLocked(s, true), nil
}

func (m *Manager) viewSubscriberLocked(s *Subscriber, includeDevices bool) SubscriberView {
	role := s.BillingRole
	if role == "" {
		role = BillingRoleTrusted
	}
	v := SubscriberView{
		ID: s.ID, Name: s.Name, AccessToken: s.AccessToken,
		Notes: s.Notes, CreatedAt: s.CreatedAt,
		BillingRole: role,
	}
	for _, c := range m.clients {
		if c.SubscriberID == s.ID {
			v.DeviceCount++
			if includeDevices {
				v.Devices = append(v.Devices, *c)
			}
		}
	}
	if includeDevices {
		sort.Slice(v.Devices, func(i, j int) bool { return v.Devices[i].CreatedAt.Before(v.Devices[j].CreatedAt) })
	}
	return v
}

// RenameSubscriber / SetNotes / SetBillingRole — light admin patches, no cascade needed.
func (m *Manager) PatchSubscriber(id string, name, notes, billingRole *string) (*Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subscribers[id]
	if !ok {
		return nil, errSubscriberNotFound
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return nil, errors.New("name cannot be empty")
		}
		s.Name = n
	}
	if notes != nil {
		s.Notes = strings.TrimSpace(*notes)
	}
	if billingRole != nil {
		role, err := normalizeBillingRole(*billingRole)
		if err != nil {
			return nil, err
		}
		s.BillingRole = role
	}
	if s.BillingRole == "" {
		s.BillingRole = BillingRoleTrusted
	}
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		return nil, err
	}
	m.fire("subscriber.patched", id, map[string]string{"name": s.Name})
	cp := *s
	if cp.BillingRole == "" {
		cp.BillingRole = "trusted"
	}
	return &cp, nil
}

// RegenerateAccessToken invalidates the old cabinet URL by issuing a new one.
// Devices, profiles, interfaces stay untouched — only the cabinet credential
// rotates.
func (m *Manager) RegenerateAccessToken(id string) (*Subscriber, error) {
	tok, err := newAccessToken()
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subscribers[id]
	if !ok {
		return nil, errSubscriberNotFound
	}
	s.AccessToken = tok
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		return nil, err
	}
	m.fire("subscriber.regen_token", id, map[string]string{"name": s.Name})
	cp := *s
	return &cp, nil
}

// DeleteSubscriber removes the subscriber AND all their devices (cascade).
// Each device's interface goes down, .conf is removed, profile entry deleted.
func (m *Manager) DeleteSubscriber(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subscribers[id]
	if !ok {
		return errSubscriberNotFound
	}

	// Collect all owned devices, drop interfaces, scrub state.
	var droppedDevices, droppedProfiles []string
	for cid, c := range m.clients {
		if c.SubscriberID != id {
			continue
		}
		droppedDevices = append(droppedDevices, cid)
		pid := c.ProfileID
		if ps, ok := m.profiles[pid]; ok {
			if err := ps.runner.Down(); err != nil {
				slog.Warn("AWG subscriber deletion interface cleanup failed", slog.String("component", "awg"), slog.String("operation", "delete_subscriber_down"), slog.String("subscriber_id", id), slog.String("profile_id", pid), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
			}
			if err := m.store.RemoveProfileConf(ps.profile.Iface); err != nil {
				slog.Warn("AWG subscriber deletion config cleanup failed", slog.String("component", "awg"), slog.String("operation", "delete_subscriber_remove_config"), slog.String("subscriber_id", id), slog.String("profile_id", pid), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
			}
			delete(m.profiles, pid)
			droppedProfiles = append(droppedProfiles, pid)
		}
		delete(m.clients, cid)
	}
	delete(m.subscribers, id)

	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		return err
	}
	m.fire("subscriber.deleted", id, map[string]any{
		"name":    s.Name,
		"devices": len(droppedDevices),
	})
	return nil
}

// FindSubscriber looks up a subscriber by ID.
func (m *Manager) FindSubscriber(id string) (*Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subscribers[id]
	if !ok {
		return nil, errSubscriberNotFound
	}
	cp := *s
	if cp.BillingRole == "" {
		cp.BillingRole = "trusted"
	}
	return &cp, nil
}

// FindSubscriberByToken is the public cabinet's auth lookup. Linear scan is
// fine: subscriber count is small.
func (m *Manager) FindSubscriberByToken(token string) (*Subscriber, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.subscribers {
		if tokenEqual(s.AccessToken, token) {
			cp := *s
			if cp.BillingRole == "" {
				cp.BillingRole = "trusted"
			}
			return &cp, nil
		}
	}
	return nil, errSubscriberNotFound
}

// CabinetSnapshot is what the public /api/cabinet/:token returns. Auth via
// token; if invalid → error. Devices include live handshake info merged in.
func (m *Manager) CabinetSnapshot(token string) (CabinetView, error) {
	m.mu.Lock()
	var subID, subName string
	for _, s := range m.subscribers {
		if tokenEqual(s.AccessToken, token) {
			subID = s.ID
			subName = s.Name
			break
		}
	}
	if subID == "" {
		m.mu.Unlock()
		return CabinetView{}, errSubscriberNotFound
	}
	devices := []CabinetDevice{}
	for _, c := range m.clients {
		if c.SubscriberID != subID {
			continue
		}
		devices = append(devices, CabinetDevice{
			ID: c.ID, Name: c.Name, Address: c.Address,
			Enabled: c.Enabled, CreatedAt: c.CreatedAt,
			LatestHandshakeAt: c.LastHandshakeAt,
		})
	}
	m.mu.Unlock()

	// Live handshake merge (outside lock — ShowDump shells out).
	pubByDevID := map[string]string{}
	m.mu.Lock()
	for _, c := range m.clients {
		if c.SubscriberID == subID {
			pubByDevID[c.ID] = c.PublicKey
		}
	}
	m.mu.Unlock()
	status := mergedDump(m.cfg.AWGBin)
	for i := range devices {
		if pk := pubByDevID[devices[i].ID]; pk != "" {
			if s, ok := status[pk]; ok && s.LatestHandshake != nil {
				devices[i].LatestHandshakeAt = s.LatestHandshake
			}
		}
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].CreatedAt.Before(devices[j].CreatedAt) })
	return CabinetView{Name: subName, Devices: devices}, nil
}

// AddDevice is invoked by both the public cabinet (subscriber-driven) and the
// admin "Add device for subscriber" flow (future). Atomically creates a
// Profile + Client bound to the subscriber, brings the interface up, returns
// the rendered .conf so the caller can deliver it.
func (m *Manager) AddDevice(subscriberID, deviceName string, spec ObfuscationSpec) (*Client, []byte, error) {
	deviceName = strings.TrimSpace(deviceName)
	if deviceName == "" {
		deviceName = "device"
	}

	cliPriv, err := m.keys.GenPrivate()
	if err != nil {
		return nil, nil, err
	}
	cliPub, err := m.keys.Public(cliPriv)
	if err != nil {
		return nil, nil, err
	}
	cliPSK, err := m.keys.GenPSK()
	if err != nil {
		return nil, nil, err
	}

	var (
		runner     Runner
		iface      string
		subName    string
		clientCopy Client
		profID     string
		renderArgs ClientRenderArgs
	)

	if err := func() error {
		m.mu.Lock()
		defer m.mu.Unlock()

		s, ok := m.subscribers[subscriberID]
		if !ok {
			return errSubscriberNotFound
		}

		profileID := fmt.Sprintf("dev-%s-%s", s.ID, uuid.NewString()[:6])
		pname := fmt.Sprintf("%s · %s", s.Name, deviceName)
		p, err := m.newProfile(profileID, pname, "Device of "+s.Name, spec)
		if err != nil {
			return fmt.Errorf("create profile: %w", err)
		}

		ps := &profileState{
			profile: p,
			runner:  Runner{AWGBin: m.cfg.AWGBin, AWGQuickBin: m.cfg.AWGQuickBin, Iface: p.Iface},
		}
		m.profiles[p.ID] = ps

		used := map[string]struct{}{}
		for _, pr := range m.profiles {
			used[pr.profile.Address] = struct{}{}
		}
		for _, existing := range m.clients {
			used[existing.Address] = struct{}{}
		}
		// Use the per-profile IPAM so each awgN gets its own /24 and kernel routes
		// don't overlap (awg0→10.8.0.x, awg1→10.8.1.x, …).
		addr, err := m.ipamForPort(p.Port).Next(used)
		if err != nil {
			delete(m.profiles, p.ID)
			return fmt.Errorf("allocate address: %w", err)
		}
		now := time.Now().UTC()
		c := &Client{
			ID: uuid.NewString(), SubscriberID: s.ID, ProfileID: p.ID,
			Name: deviceName, Address: addr,
			PrivateKey: cliPriv, PublicKey: cliPub, PreSharedKey: cliPSK,
			Enabled:   true,
			CreatedAt: now, UpdatedAt: now,
		}
		m.clients[c.ID] = c

		if err := m.persistAllLocked(); err != nil {
			delete(m.clients, c.ID)
			delete(m.profiles, p.ID)
			return fmt.Errorf("persist: %w", err)
		}

		runner = ps.runner
		iface = p.Iface
		subName = s.Name
		clientCopy = *c
		profID = p.ID
		renderArgs = m.renderArgs(p, c)
		return nil
	}(); err != nil {
		return nil, nil, err
	}

	// awg-quick outside the lock — never block ListSubscribers on interface bring-up.
	_ = runner.Down()
	if err := runner.Up(); err != nil {
		m.rollbackDevice(clientCopy.ID, profID, iface, true)
		return nil, nil, fmt.Errorf("awg-quick up %s: %w", iface, err)
	}

	conf, err := RenderClient(renderArgs)
	if err != nil {
		_ = runner.Down()
		m.rollbackDevice(clientCopy.ID, profID, iface, true)
		return nil, nil, fmt.Errorf("render client: %w", err)
	}

	m.mu.Lock()
	m.fire("device.created", clientCopy.ID, map[string]string{
		"name": clientCopy.Name, "subscriberId": clientCopy.SubscriberID, "subscriberName": subName, "address": clientCopy.Address,
	})
	m.mu.Unlock()
	return &clientCopy, conf, nil
}

func (m *Manager) rollbackDevice(clientID, profileID, iface string, removeConf bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, clientID)
	delete(m.profiles, profileID)
	if removeConf {
		_ = m.store.RemoveProfileConf(iface)
	}
	_ = m.persistAllLocked()
}

// DeleteDevice removes one device (Client + its dedicated Profile + iface)
// without touching the parent subscriber. Used both by admin and the cabinet.
// If actorSubID is non-empty, the device must belong to that subscriber —
// guards the public cabinet against cross-subscriber tampering.
func (m *Manager) DeleteDevice(deviceID, actorSubID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.clients[deviceID]
	if !ok {
		return errNotFound
	}
	if actorSubID != "" && c.SubscriberID != actorSubID {
		return errNotFound
	}
	pid := c.ProfileID
	subID := c.SubscriberID
	name := c.Name
	delete(m.clients, deviceID)
	if ps, ok := m.profiles[pid]; ok {
		if err := ps.runner.Down(); err != nil {
			slog.Warn("AWG device deletion interface cleanup failed", slog.String("component", "awg"), slog.String("operation", "delete_device_down"), slog.String("device_id", deviceID), slog.String("profile_id", pid), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
		}
		if err := m.store.RemoveProfileConf(ps.profile.Iface); err != nil {
			slog.Warn("AWG device deletion config cleanup failed", slog.String("component", "awg"), slog.String("operation", "delete_device_remove_config"), slog.String("device_id", deviceID), slog.String("profile_id", pid), slog.String("interface", ps.profile.Iface), slog.Any("error", err))
		}
		delete(m.profiles, pid)
	}
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		return err
	}
	m.fire("device.deleted", deviceID, map[string]string{
		"name": name, "subscriberId": subID,
	})
	return nil
}

var errSubscriberNotFound = errors.New("subscriber not found")

func IsSubscriberNotFound(err error) bool { return errors.Is(err, errSubscriberNotFound) }
