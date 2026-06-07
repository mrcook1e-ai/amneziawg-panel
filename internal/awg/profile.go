package awg

import "time"

// Profile is a single AmneziaWG 2.0 interface (awgN) on its own UDP port with
// its own server keypair and obfuscation params. Clients are attached to
// exactly one profile and inherit endpoint/server-pubkey/obfuscation from it.
//
// Obfuscation parameters MUST match between server and every client of the
// profile, otherwise handshake fails. The panel is the single source of truth:
// it generates J/S/H/Itime and renders both the server awgN.conf and each
// client .conf from the same Profile record.
//
// I1..I5 and J1..J3 are opaque CPS strings — pasted by the admin from an
// external generator (e.g. AmneziaWG-Architect). Empty means no CPS for that
// slot. Itime = 0 disables the CPS chain entirely.
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
	S1 int `json:"s1"`
	S2 int `json:"s2"`
	S3 int `json:"s3"`
	S4 int `json:"s4"`

	// Magic header ranges, format "min-max". Ranges must not overlap.
	H1 string `json:"h1"`
	H2 string `json:"h2"`
	H3 string `json:"h3"`
	H4 string `json:"h4"`

	// CPS signature chain (up to 5 packets) and custom junk packets in the
	// CPS train (up to 3). Opaque strings in CPS tag syntax. Empty = unused.
	I1 string `json:"i1,omitempty"`
	I2 string `json:"i2,omitempty"`
	I3 string `json:"i3,omitempty"`
	I4 string `json:"i4,omitempty"`
	I5 string `json:"i5,omitempty"`
	J1 string `json:"j1,omitempty"`
	J2 string `json:"j2,omitempty"`
	J3 string `json:"j3,omitempty"`

	// Interval in seconds for sending the CPS chain. 0 disables.
	Itime int `json:"itime"`

	CreatedAt time.Time `json:"createdAt"`
}
