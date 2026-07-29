package awg

import (
	"strings"
	"testing"
)

func testProfile() *Profile {
	return &Profile{
		ID:         "p1",
		Name:       "test",
		Iface:      "awg0",
		Port:       51820,
		PrivateKey: "server-priv",
		PublicKey:  "server-pub",
		Address:    "10.8.0.1",
		Jc:         5,
		Jmin:       64,
		Jmax:       512,
		S1:         15,
		S2:         20,
		S3:         10,
		S4:         8,
		H1:         "100-200",
		H2:         "300-400",
		H3:         "500-600",
		H4:         "700-800",
		I1:         "<b 0xc0><r 16><t>",
		I2:         "<r 32>",
	}
}

func TestRenderProfile_OmitsIPackets(t *testing.T) {
	// Server is responder — I* is initiator-only; must not appear in server conf.
	out, err := RenderProfile(ProfileRenderArgs{
		Profile:    testProfile(),
		Peers:      nil,
		SubnetCIDR: "10.8.0.0/24",
		Egress:     "eth0",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, key := range []string{"I1", "I2", "I3", "I4", "I5"} {
		if strings.Contains(s, key+" =") || strings.Contains(s, key+"=") {
			t.Fatalf("server conf must not emit %s:\n%s", key, s)
		}
	}
	if !strings.Contains(s, "Jc = 5") || !strings.Contains(s, "S4 = 8") {
		t.Fatalf("missing junk/S params:\n%s", s)
	}
}

func TestRenderClient_IncludesIPackets(t *testing.T) {
	p := testProfile()
	c := &Client{
		ID:         "c1",
		Name:       "laptop",
		Address:    "10.8.0.2",
		PrivateKey: "client-priv",
		PublicKey:  "client-pub",
		Enabled:    true,
	}
	out, err := RenderClient(ClientRenderArgs{
		Profile:    p,
		Client:     c,
		DNS:        "1.1.1.1",
		MTU:        1280,
		AllowedIPs: "0.0.0.0/0, ::/0",
		Endpoint:   "vpn.example:51820",
		Keepalive:  25,
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "I1 = <b 0xc0><r 16><t>") {
		t.Fatalf("client conf missing I1:\n%s", s)
	}
	if !strings.Contains(s, "I2 = <r 32>") {
		t.Fatalf("client conf missing I2:\n%s", s)
	}
	if !strings.Contains(s, "Endpoint = vpn.example:51820") {
		t.Fatalf("missing endpoint:\n%s", s)
	}
}
