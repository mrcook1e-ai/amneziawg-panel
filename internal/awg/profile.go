package awg

import "time"

// Profile is a single AmneziaWG interface (awgN) on its own UDP port with its
// own server keypair and obfuscation params. Clients are attached to exactly
// one profile and inherit endpoint/server-pubkey/obfuscation from it.
//
// AWG 1.0 fields (Jc/Jmin/Jmax/S1/S2/H1-H4) are always set. AWG 1.5 fields
// (I1..I5) are opaque CPS strings — pasted by the admin from an external
// generator (AmneziaWG-Architect). Empty I1..I5 → behaves as a 1.0 endpoint.
type Profile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Iface       string `json:"iface"`
	Port        int    `json:"port"`

	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	Address    string `json:"address"`

	Jc   int    `json:"jc"`
	Jmin int    `json:"jmin"`
	Jmax int    `json:"jmax"`
	S1   int    `json:"s1"`
	S2   int    `json:"s2"`
	H1   string `json:"h1"`
	H2   string `json:"h2"`
	H3   string `json:"h3"`
	H4   string `json:"h4"`

	I1 string `json:"i1,omitempty"`
	I2 string `json:"i2,omitempty"`
	I3 string `json:"i3,omitempty"`
	I4 string `json:"i4,omitempty"`
	I5 string `json:"i5,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}
