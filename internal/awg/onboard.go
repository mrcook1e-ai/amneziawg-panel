package awg

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// OnboardToken is a one-shot invitation an admin issues for a single client to
// self-configure. Redemption atomically creates one profile + one client bound
// to that profile (1:1 model — every onboarded user gets their own awgN
// interface, since obfuscation params differ per client).
type OnboardToken struct {
	ID        string     `json:"id"`        // short admin-facing slug
	Token     string     `json:"token"`     // 256-bit url-safe secret in the link
	Name      string     `json:"name"`      // admin's label, e.g. "Vasya laptop"
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"` // nil = no expiry
	UsedAt    *time.Time `json:"usedAt,omitempty"`

	// Set after successful redemption — used to surface the created identity
	// to admin UI and to refuse re-redemption.
	CreatedProfileID string `json:"createdProfileId,omitempty"`
	CreatedClientID  string `json:"createdClientId,omitempty"`
}

// Status is the derived state shown in the admin list.
type TokenStatus string

const (
	TokenPending TokenStatus = "pending"
	TokenUsed    TokenStatus = "used"
	TokenExpired TokenStatus = "expired"
)

func (t *OnboardToken) Status(now time.Time) TokenStatus {
	if t.UsedAt != nil {
		return TokenUsed
	}
	if t.ExpiresAt != nil && now.After(*t.ExpiresAt) {
		return TokenExpired
	}
	return TokenPending
}

// TokenView is the admin-facing projection. The raw `Token` field is included
// so the admin can copy the invite URL.
type TokenView struct {
	ID               string      `json:"id"`
	Token            string      `json:"token"`
	Name             string      `json:"name"`
	CreatedAt        time.Time   `json:"createdAt"`
	ExpiresAt        *time.Time  `json:"expiresAt,omitempty"`
	UsedAt           *time.Time  `json:"usedAt,omitempty"`
	Status           TokenStatus `json:"status"`
	CreatedClientID  string      `json:"createdClientId,omitempty"`
	CreatedProfileID string      `json:"createdProfileId,omitempty"`
}

// PublicTokenStatus is what the unauthenticated onboard page sees on GET. We
// deliberately do NOT leak the admin's label or any client info.
type PublicTokenStatus struct {
	Valid bool `json:"valid"` // not used and not expired
	Used  bool `json:"used"`
}

// RedeemResult is what the unauthenticated onboard page receives on successful
// POST: the rendered .conf and the base64-PNG QR. Single delivery — token is
// burned, refreshing the page won't get them back.
type RedeemResult struct {
	Conf    string `json:"conf"`
	QRPng64 string `json:"qrPng64"` // raw base64 (caller wraps as data: URL)
}

func newTokenSecret() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// CreateToken issues a new invite. ttl == 0 means "no expiry" (the admin can
// always revoke manually). Returns the freshly-minted token with its secret.
func (m *Manager) CreateToken(name string, ttl time.Duration) (*OnboardToken, error) {
	secret, err := newTokenSecret()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	t := &OnboardToken{
		ID:        uuid.NewString()[:8],
		Token:     secret,
		Name:      strings.TrimSpace(name),
		CreatedAt: now,
	}
	if ttl > 0 {
		exp := now.Add(ttl)
		t.ExpiresAt = &exp
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tokens == nil {
		m.tokens = map[string]*OnboardToken{}
	}
	m.tokens[t.ID] = t
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		delete(m.tokens, t.ID)
		return nil, err
	}
	m.fire("token.created", t.ID, map[string]string{"name": t.Name})
	return t, nil
}

func (m *Manager) ListTokens() []TokenView {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	out := make([]TokenView, 0, len(m.tokens))
	for _, t := range m.tokens {
		out = append(out, TokenView{
			ID: t.ID, Token: t.Token, Name: t.Name,
			CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, UsedAt: t.UsedAt,
			Status:           t.Status(now),
			CreatedClientID:  t.CreatedClientID,
			CreatedProfileID: t.CreatedProfileID,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

func (m *Manager) RevokeToken(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tokens[id]; !ok {
		return errTokenNotFound
	}
	delete(m.tokens, id)
	if err := m.store.SaveState(m.dumpStateLocked()); err != nil {
		return err
	}
	m.fire("token.revoked", id, nil)
	return nil
}

// findTokenLocked looks up by the secret (not admin ID). Linear scan is fine
// — token count is small in practice and this is a one-shot path.
func (m *Manager) findTokenLocked(secret string) *OnboardToken {
	for _, t := range m.tokens {
		if t.Token == secret {
			return t
		}
	}
	return nil
}

// TokenStatusPublic is the read-only check the onboard page performs before
// showing its form. Returns valid=false for unknown, used, or expired.
func (m *Manager) TokenStatusPublic(secret string) PublicTokenStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.findTokenLocked(secret)
	if t == nil {
		return PublicTokenStatus{Valid: false}
	}
	now := time.Now().UTC()
	switch t.Status(now) {
	case TokenUsed:
		return PublicTokenStatus{Valid: false, Used: true}
	case TokenExpired:
		return PublicTokenStatus{Valid: false}
	default:
		return PublicTokenStatus{Valid: true}
	}
}

// RedeemToken is the public POST handler's core: parse the snippet, create
// profile+client atomically, mark token used, return the rendered .conf. The
// caller is expected to render the QR from `client.PublicKey` -- actually
// no, callers want the full .conf as a QR, so the handler does PNG encoding.
//
// `endpoint` is the server-side resolved host:port used in the .conf [Peer]
// block, since Manager.renderArgs already does this from cfg.WGHost.
func (m *Manager) RedeemToken(secret, clientName string, spec ObfuscationSpec) (*Client, []byte, error) {
	clientName = strings.TrimSpace(clientName)
	if clientName == "" {
		clientName = "client"
	}

	// Pre-generate keys outside the lock to keep critical section short.
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

	m.mu.Lock()
	defer m.mu.Unlock()

	t := m.findTokenLocked(secret)
	if t == nil {
		return nil, nil, errTokenNotFound
	}
	now := time.Now().UTC()
	if t.UsedAt != nil {
		return nil, nil, errTokenUsed
	}
	if t.ExpiresAt != nil && now.After(*t.ExpiresAt) {
		return nil, nil, errTokenExpired
	}

	// Profile: ID derived from token ID so admin can correlate.
	profileID := "onb-" + t.ID
	pname := clientName
	if t.Name != "" {
		pname = t.Name
	}
	p, err := m.newProfile(profileID, pname, "Onboarded via invite "+t.ID, spec)
	if err != nil {
		return nil, nil, fmt.Errorf("create profile: %w", err)
	}

	ps := &profileState{
		profile: p,
		runner:  Runner{AWGBin: m.cfg.AWGBin, AWGQuickBin: m.cfg.AWGQuickBin, Iface: p.Iface},
	}
	m.profiles[p.ID] = ps

	// Client: allocate an IP in the shared subnet, attach to the new profile.
	used := map[string]struct{}{}
	for _, pr := range m.profiles {
		used[pr.profile.Address] = struct{}{}
	}
	for _, c := range m.clients {
		used[c.Address] = struct{}{}
	}
	addr, err := m.ipam.Next(used)
	if err != nil {
		delete(m.profiles, p.ID)
		return nil, nil, fmt.Errorf("allocate address: %w", err)
	}
	c := &Client{
		ID: uuid.NewString(), ProfileID: p.ID,
		Name: clientName, Address: addr,
		PrivateKey: cliPriv, PublicKey: cliPub, PreSharedKey: cliPSK,
		Enabled:   true,
		CreatedAt: now, UpdatedAt: now,
	}
	m.clients[c.ID] = c

	t.UsedAt = &now
	t.CreatedProfileID = p.ID
	t.CreatedClientID = c.ID

	if err := m.persistAllLocked(); err != nil {
		delete(m.clients, c.ID)
		delete(m.profiles, p.ID)
		t.UsedAt = nil
		t.CreatedProfileID = ""
		t.CreatedClientID = ""
		return nil, nil, fmt.Errorf("persist: %w", err)
	}
	_ = ps.runner.Down()
	if err := ps.runner.Up(); err != nil {
		delete(m.clients, c.ID)
		delete(m.profiles, p.ID)
		t.UsedAt = nil
		t.CreatedProfileID = ""
		t.CreatedClientID = ""
		_ = m.persistAllLocked()
		return nil, nil, fmt.Errorf("awg-quick up %s: %w", p.Iface, err)
	}

	conf, err := RenderClient(m.renderArgs(p, c))
	if err != nil {
		return nil, nil, err
	}

	m.fire("token.redeemed", t.ID, map[string]string{"clientId": c.ID, "profileId": p.ID, "name": c.Name})
	m.fire("profile.created", p.ID, map[string]any{"name": p.Name, "iface": p.Iface, "port": p.Port})
	m.fire("client.created", c.ID, map[string]string{"name": c.Name, "address": c.Address, "profileId": p.ID, "onboarded": "true"})
	return c, conf, nil
}

var (
	errTokenNotFound = errors.New("invite not found")
	errTokenUsed     = errors.New("invite already used")
	errTokenExpired  = errors.New("invite expired")
)

func IsTokenNotFound(err error) bool { return errors.Is(err, errTokenNotFound) }
func IsTokenUsed(err error) bool     { return errors.Is(err, errTokenUsed) }
func IsTokenExpired(err error) bool  { return errors.Is(err, errTokenExpired) }
