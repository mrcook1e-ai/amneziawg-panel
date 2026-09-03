package awg

import (
	"bytes"
	"strconv"
	"text/template"
	"time"
)

// Client is a single VPN device owned by a Subscriber. The Go-level naming
// stays "Client" for historical reasons; the user-facing terminology is
// "устройство" (device). One Client = one Profile = one awgN interface,
// because obfuscation params are bound to the interface, not the subscriber.
type Client struct {
	ID               string    `json:"id"`
	SubscriberID     string    `json:"subscriberId"`
	ProfileID        string    `json:"profileId"`
	Name             string    `json:"name"`
	Address          string    `json:"address"`
	PrivateKey       string    `json:"privateKey"`
	PublicKey        string    `json:"publicKey"`
	PreSharedKey     string    `json:"preSharedKey"`
	Enabled          bool      `json:"enabled"`
	BillingSuspended bool      `json:"billingSuspended,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`

	Notes     string     `json:"notes,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`

	DNSOverride        string `json:"dnsOverride,omitempty"`
	AllowedIPsOverride string `json:"allowedIPsOverride,omitempty"`
	MTUOverride        int    `json:"mtuOverride,omitempty"`

	TotalRx         uint64     `json:"totalRx,omitempty"`
	TotalTx         uint64     `json:"totalTx,omitempty"`
	LastHandshakeAt *time.Time `json:"lastHandshakeAt,omitempty"`
}

// Schema versions of state.json.
//
//   - v4 introduced Subscriber as the top-level account entity; each Client
//     (= device) carries SubscriberID.
//   - v5 adds the optional AWG 3.x profile fields and drops the dead AWG 1.5
//     beta fields (Itime, J1-J3).
//
// v5 is a purely additive change, so a v4 store loads as-is and is rewritten
// as v5 on the next Save. Anything below MinSchemaVersion is refused
// (fail-fast, no in-place migration); anything above SchemaVersion is refused
// too, so an accidental binary downgrade cannot silently drop fields it does
// not know about and persist the loss.
const (
	SchemaVersion    = 5
	MinSchemaVersion = 4
)

type Config struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Subscribers   map[string]*Subscriber `json:"subscribers"`
	Profiles      map[string]*Profile    `json:"profiles"`
	Clients       map[string]*Client     `json:"clients"`
}

// NOTE on Itime / J1-J3: they belonged to the abandoned AWG 1.5 beta and are
// gone from both amneziawg-go v3 and amneziawg-tools v3.1. amneziawg-tools
// still `goto error`s on any unrecognised [Interface] key, so emitting them
// would break `awg setconf` outright. ParseObfuscation accepts and drops them
// so stale admin snippets keep working; nothing stores or renders them.
//
// NOTE on I1–I5: amneziawg-go sends signature packets only when initiating a
// handshake. Official docs mark them client-side only (need not match on the
// responder). We still store them on the Profile (source of truth for client
// exports) but do NOT emit them on the server profile template — the server
// is almost always the responder. Client configs keep I* when set.
//
// NOTE on the AWG 3.x keys: they are emitted only when actually set, so an
// AWG 1.0/2.0 profile renders byte-for-byte as it did before this generation
// existed and its conf stays loadable by pre-3.x amneziawg-tools.

var profileTmpl = template.Must(template.New("profile").Parse(`# Managed by amneziawg-panel. Do not edit by hand.

[Interface]
PrivateKey = {{.Profile.PrivateKey}}
Address = {{.Profile.Address}}/24
ListenPort = {{.Profile.Port}}
PostUp = iptables -I FORWARD -i %i -j ACCEPT; iptables -I FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -s {{.SubnetCIDR}} -o {{.Egress}} -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -s {{.SubnetCIDR}} -o {{.Egress}} -j MASQUERADE
Jc = {{.Profile.Jc}}
Jmin = {{.Profile.Jmin}}
Jmax = {{.Profile.Jmax}}
S1 = {{.Profile.S1}}
S2 = {{.Profile.S2}}
{{if .EmitS34}}S3 = {{.Profile.S3}}
S4 = {{.Profile.S4}}
{{end}}H1 = {{.Profile.H1}}
H2 = {{.Profile.H2}}
H3 = {{.Profile.H3}}
H4 = {{.Profile.H4}}
{{if .Profile.HeaderProtectionKey}}HeaderProtectionKey = {{.Profile.HeaderProtectionKey}}
{{end}}{{if .Profile.RandomTrailers}}RandomTrailers = on
{{end}}{{if .Profile.DisableCookies}}DisableCookies = on
{{end}}{{range .Peers}}
# {{.Name}} ({{.ID}})
[Peer]
PublicKey = {{.PublicKey}}
{{if .PreSharedKey}}PresharedKey = {{.PreSharedKey}}
{{end}}AllowedIPs = {{.Address}}/32
{{end}}`))

var clientTmpl = template.Must(template.New("client").Parse(`[Interface]
PrivateKey = {{.Client.PrivateKey}}
Address = {{.Client.Address}}/24
{{if .DNS}}DNS = {{.DNS}}
{{end}}{{if .MTU}}MTU = {{.MTU}}
{{end}}Jc = {{.Profile.Jc}}
Jmin = {{.Profile.Jmin}}
Jmax = {{.Profile.Jmax}}
S1 = {{.Profile.S1}}
S2 = {{.Profile.S2}}
{{if .EmitS34}}S3 = {{.Profile.S3}}
S4 = {{.Profile.S4}}
{{end}}H1 = {{.Profile.H1}}
H2 = {{.Profile.H2}}
H3 = {{.Profile.H3}}
H4 = {{.Profile.H4}}
{{if .Profile.I1}}I1 = {{.Profile.I1}}
{{end}}{{if .Profile.I2}}I2 = {{.Profile.I2}}
{{end}}{{if .Profile.I3}}I3 = {{.Profile.I3}}
{{end}}{{if .Profile.I4}}I4 = {{.Profile.I4}}
{{end}}{{if .Profile.I5}}I5 = {{.Profile.I5}}
{{end}}{{if .Profile.HeaderProtectionKey}}HeaderProtectionKey = {{.Profile.HeaderProtectionKey}}
{{end}}{{if .Profile.RandomTrailers}}RandomTrailers = on
{{end}}{{if .Profile.DisableCookies}}DisableCookies = on
{{end}}{{if .Profile.ContentPaddingAddition}}ContentPaddingAddition = {{.Profile.ContentPaddingAddition}}
{{end}}{{if .Profile.RekeyAfterTime}}RekeyAfterTime = {{.Profile.RekeyAfterTime}}
{{end}}{{if .Profile.RekeyTimeout}}RekeyTimeout = {{.Profile.RekeyTimeout}}
{{end}}{{if .Profile.RejectAfterTime}}RejectAfterTime = {{.Profile.RejectAfterTime}}
{{end}}{{if .Profile.KeepaliveTimeout}}KeepaliveTimeout = {{.Profile.KeepaliveTimeout}}
{{end}}{{if .Profile.MaxHandshakeAttempts}}MaxHandshakeAttempts = {{.Profile.MaxHandshakeAttempts}}
{{end}}
[Peer]
PublicKey = {{.Profile.PublicKey}}
{{if .Client.PreSharedKey}}PresharedKey = {{.Client.PreSharedKey}}
{{end}}AllowedIPs = {{.AllowedIPs}}
{{if .Keepalive}}PersistentKeepalive = {{.Keepalive}}
{{end}}Endpoint = {{.Endpoint}}
`))

// emitS34 reports whether S3/S4 belong in a rendered conf. AWG 1.0 predates
// both keys and its parsers abort on anything unrecognised, so a profile
// classified as 1.0 must not carry them.
func emitS34(p *Profile) bool {
	return p != nil && p.Generation() != GenAWG1
}

type ProfileRenderArgs struct {
	Profile    *Profile
	Peers      []*Client
	SubnetCIDR string
	Egress     string
	// EmitS34 is derived from Profile; set by RenderProfile.
	EmitS34 bool
}

func RenderProfile(a ProfileRenderArgs) ([]byte, error) {
	a.EmitS34 = emitS34(a.Profile)
	var buf bytes.Buffer
	err := profileTmpl.Execute(&buf, a)
	return buf.Bytes(), err
}

type ClientRenderArgs struct {
	Profile    *Profile
	Client     *Client
	DNS        string
	MTU        int
	AllowedIPs string
	Endpoint   string
	// KeepaliveSecs is the server-wide fallback in seconds. A profile-level
	// PersistentKeepalive range (AWG 3.1) takes precedence over it.
	KeepaliveSecs int

	// Derived by RenderClient — do not set from callers.
	Keepalive string
	EmitS34   bool
}

func RenderClient(a ClientRenderArgs) ([]byte, error) {
	if a.Client != nil {
		if a.Client.DNSOverride != "" {
			a.DNS = a.Client.DNSOverride
		}
		if a.Client.AllowedIPsOverride != "" {
			a.AllowedIPs = a.Client.AllowedIPsOverride
		}
		if a.Client.MTUOverride > 0 {
			a.MTU = a.Client.MTUOverride
		}
	}
	a.EmitS34 = emitS34(a.Profile)
	a.Keepalive = resolveKeepalive(a.Profile, a.KeepaliveSecs)
	var buf bytes.Buffer
	err := clientTmpl.Execute(&buf, a)
	return buf.Bytes(), err
}

// resolveKeepalive picks the peer-section PersistentKeepalive value: the
// profile's range when set (AWG 3.1 randomises it), otherwise the server-wide
// integer. Empty result means the key is omitted entirely.
func resolveKeepalive(p *Profile, fallbackSecs int) string {
	if p != nil && p.PersistentKeepalive != "" {
		return p.PersistentKeepalive
	}
	if fallbackSecs > 0 {
		return strconv.Itoa(fallbackSecs)
	}
	return ""
}
