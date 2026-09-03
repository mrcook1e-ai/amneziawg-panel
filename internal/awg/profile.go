package awg

import "time"

// Profile is a single AmneziaWG interface (awgN) on its own UDP port with its
// own server keypair and obfuscation params. Clients are attached to exactly
// one profile and inherit endpoint/server-pubkey/obfuscation from it.
//
// A profile carries one of three generations of parameters (see Generation):
// AWG 1.0 (Jc/S1-S2/H1-H4 fixed), AWG 2.0 (+ S3/S4, H as ranges, I1-I5), or
// AWG 3.1 (+ HeaderProtectionKey, ContentPaddingAddition, randomised timers,
// RandomTrailers, DisableCookies). The pinned amneziawg-go v3 serves all three
// from one binary: with the 3.x fields unset it is byte-for-byte a 2.0/1.0
// device on the wire, so older profiles keep working untouched.
//
// Which side must match:
//
//   - must match on server AND every client: S1-S4, H1-H4,
//     HeaderProtectionKey, RandomTrailers
//   - initiator-only / one-sided (safe to vary per client): Jc/Jmin/Jmax,
//     I1-I5, ContentPaddingAddition, the Rekey*/Reject*/Keepalive*/
//     MaxHandshakeAttempts timers, PersistentKeepalive
//
// I1..I5 are opaque CPS strings — pasted by the admin from an external
// generator or produced by the cabinet preset generator. Empty means no CPS
// for that slot.
type Profile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Iface       string `json:"iface"`
	Port        int    `json:"port"`

	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	Address    string `json:"address"`

	// Junk train sent before handshake.
	Jc   int `json:"jc"`
	Jmin int `json:"jmin"`
	Jmax int `json:"jmax"`

	// Random padding lengths added to each packet type.
	// Invariants: S1+56 != S2, S1+56 != S3, S2+92 != S3.
	// With HeaderProtectionKey set, all four must be >= 12 (ChaCha20 nonce
	// is read from the first 12 bytes of the S prefix).
	S1 int `json:"s1"`
	S2 int `json:"s2"`
	S3 int `json:"s3"`
	S4 int `json:"s4"`

	// Magic header values, either "n" (fixed, AWG 1.0 / 3.1 style) or
	// "min-max" (range, AWG 2.0 style). Ranges must not overlap.
	H1 string `json:"h1"`
	H2 string `json:"h2"`
	H3 string `json:"h3"`
	H4 string `json:"h4"`

	// CPS signature chain (up to 5 packets). Opaque strings in CPS tag
	// syntax, initiator-only. Empty = unused.
	I1 string `json:"i1,omitempty"`
	I2 string `json:"i2,omitempty"`
	I3 string `json:"i3,omitempty"`
	I4 string `json:"i4,omitempty"`
	I5 string `json:"i5,omitempty"`

	// ── AWG 3.x ────────────────────────────────────────────────────────
	// Every field below is optional; empty/false means "not set", which is
	// exactly the 2.0 behaviour. They are emitted to .conf only when set,
	// so awg1/awg2 profiles render byte-for-byte as before and stay
	// loadable by pre-3.x amneziawg-tools.

	// HeaderProtectionKey is a base64 32-byte key (awg genkey) enabling
	// ChaCha20 encryption of service headers. Must match on both sides.
	HeaderProtectionKey string `json:"headerProtectionKey,omitempty"`
	// ContentPaddingAddition randomises padding inside the encrypted
	// payload of transport packets. Range "lo-hi" or fixed "n". One-sided.
	ContentPaddingAddition string `json:"contentPaddingAddition,omitempty"`

	// Randomised protocol timers, seconds, "lo-hi" or "n". Local behaviour.
	RekeyAfterTime       string `json:"rekeyAfterTime,omitempty"`
	RekeyTimeout         string `json:"rekeyTimeout,omitempty"`
	RejectAfterTime      string `json:"rejectAfterTime,omitempty"`
	KeepaliveTimeout     string `json:"keepaliveTimeout,omitempty"`
	MaxHandshakeAttempts string `json:"maxHandshakeAttempts,omitempty"`

	// RandomTrailers appends a random tail to handshake init/response/
	// cookie-reply. Must match: a receiver with it off drops oversized
	// packets outright.
	RandomTrailers bool `json:"randomTrailers,omitempty"`
	// DisableCookies turns off the cookie-reply/underload mechanism.
	DisableCookies bool `json:"disableCookies,omitempty"`

	// PersistentKeepalive is the peer-section keepalive as a range
	// ("25-35") or fixed value. Empty = fall back to the server-wide
	// WG_PERSISTENT_KEEPALIVE integer.
	PersistentKeepalive string `json:"persistentKeepalive,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// Protocol generation labels, as reported to the UI and used for the
// vpn:// protocol_version field.
const (
	GenAWG1  = "1.0"
	GenAWG15 = "1.5"
	GenAWG2  = "2.0"
	GenAWG31 = "3.1"
)

// Generation classifies a profile the same way the official AmneziaVPN 5.x
// client does when importing a third-party config (awgProtocolConfig.cpp),
// so a badge in the panel matches what the client will actually negotiate.
// Order matters: the first matching marker wins.
func (p *Profile) Generation() string {
	switch {
	case p.HeaderProtectionKey != "" || p.ContentPaddingAddition != "" ||
		p.RekeyAfterTime != "" || p.RekeyTimeout != "" ||
		p.RejectAfterTime != "" || p.KeepaliveTimeout != "" ||
		p.MaxHandshakeAttempts != "" ||
		p.RandomTrailers || p.DisableCookies:
		return GenAWG31
	case p.S3 != 0 || p.S4 != 0 || isRangeValue(p.H1) || isRangeValue(p.H2) ||
		isRangeValue(p.H3) || isRangeValue(p.H4):
		return GenAWG2
	case p.I1 != "" || p.I2 != "" || p.I3 != "" || p.I4 != "" || p.I5 != "":
		return GenAWG15
	default:
		return GenAWG1
	}
}
