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

func testClient() *Client {
	return &Client{
		ID:         "c1",
		Name:       "laptop",
		Address:    "10.8.0.2",
		PrivateKey: "client-priv",
		PublicKey:  "client-pub",
		Enabled:    true,
	}
}

func renderTestClient(t *testing.T, p *Profile) string {
	t.Helper()
	out, err := RenderClient(ClientRenderArgs{
		Profile:       p,
		Client:        testClient(),
		DNS:           "1.1.1.1",
		MTU:           1280,
		AllowedIPs:    "0.0.0.0/0, ::/0",
		Endpoint:      "vpn.example:51820",
		KeepaliveSecs: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func renderTestProfile(t *testing.T, p *Profile) string {
	t.Helper()
	out, err := RenderProfile(ProfileRenderArgs{
		Profile:    p,
		Peers:      []*Client{testClient()},
		SubnetCIDR: "10.8.0.0/24",
		Egress:     "eth0",
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestRenderProfile_OmitsIPackets(t *testing.T) {
	// Server is responder — I* is initiator-only; must not appear in server conf.
	s := renderTestProfile(t, testProfile())
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
	s := renderTestClient(t, testProfile())
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

// TestRenderAWG2_GoldenUnchanged is the regression gate for the AWG 3.1
// migration: an existing AWG 2.0 profile must render byte-for-byte what it
// rendered before the 3.x keys existed. Anything else silently changes the
// wire behaviour of every deployed profile on the next resync.
func TestRenderAWG2_GoldenUnchanged(t *testing.T) {
	const wantClient = `[Interface]
PrivateKey = client-priv
Address = 10.8.0.2/24
DNS = 1.1.1.1
MTU = 1280
Jc = 5
Jmin = 64
Jmax = 512
S1 = 15
S2 = 20
S3 = 10
S4 = 8
H1 = 100-200
H2 = 300-400
H3 = 500-600
H4 = 700-800
I1 = <b 0xc0><r 16><t>
I2 = <r 32>

[Peer]
PublicKey = server-pub
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
Endpoint = vpn.example:51820
`
	if got := renderTestClient(t, testProfile()); got != wantClient {
		t.Fatalf("client conf drifted.\n--- got ---\n%s\n--- want ---\n%s", got, wantClient)
	}

	const wantServer = `# Managed by amneziawg-panel. Do not edit by hand.

[Interface]
PrivateKey = server-priv
Address = 10.8.0.1/24
ListenPort = 51820
PostUp = iptables -I FORWARD -i %i -j ACCEPT; iptables -I FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE
Jc = 5
Jmin = 64
Jmax = 512
S1 = 15
S2 = 20
S3 = 10
S4 = 8
H1 = 100-200
H2 = 300-400
H3 = 500-600
H4 = 700-800

# laptop (c1)
[Peer]
PublicKey = client-pub
AllowedIPs = 10.8.0.2/32
`
	if got := renderTestProfile(t, testProfile()); got != wantServer {
		t.Fatalf("server conf drifted.\n--- got ---\n%s\n--- want ---\n%s", got, wantServer)
	}
}

func TestRenderAWG1_OmitsPost1Keys(t *testing.T) {
	p := testProfile()
	// An AWG 1.0 profile: fixed headers, no S3/S4, no CPS.
	p.S3, p.S4 = 0, 0
	p.H1, p.H2, p.H3, p.H4 = "1001", "1002", "1003", "1004"
	p.I1, p.I2 = "", ""
	if got := p.Generation(); got != GenAWG1 {
		t.Fatalf("test profile is not AWG 1.0: %q", got)
	}
	for _, conf := range []string{renderTestProfile(t, p), renderTestClient(t, p)} {
		if strings.Contains(conf, "S3") || strings.Contains(conf, "S4") {
			t.Fatalf("AWG 1.0 conf must not emit S3/S4:\n%s", conf)
		}
		if !strings.Contains(conf, "S1 = 15") || !strings.Contains(conf, "H1 = 1001") {
			t.Fatalf("AWG 1.0 conf lost its 1.0 params:\n%s", conf)
		}
	}
}

func TestRenderAWG31_KeyPlacement(t *testing.T) {
	p := testProfile()
	p.S1, p.S2, p.S3, p.S4 = 100, 120, 30, 12
	p.H1, p.H2, p.H3, p.H4 = "1", "2", "3", "4"
	p.HeaderProtectionKey = "OjW5s9DDbnR/oPuMvHwOoHFHNXBhLUXcC0Wj4bDCOWQ="
	p.ContentPaddingAddition = "10-100"
	p.RekeyAfterTime = "100-120"
	p.RekeyTimeout = "3-7"
	p.RejectAfterTime = "150-180"
	p.KeepaliveTimeout = "5-15"
	p.MaxHandshakeAttempts = "15-20"
	p.RandomTrailers = true
	p.DisableCookies = true
	p.PersistentKeepalive = "25-35"

	server := renderTestProfile(t, p)
	client := renderTestClient(t, p)

	// Must-match on both sides, or the handshake never completes.
	for _, key := range []string{
		"HeaderProtectionKey = OjW5s9DDbnR/oPuMvHwOoHFHNXBhLUXcC0Wj4bDCOWQ=",
		"RandomTrailers = on",
		"DisableCookies = on",
	} {
		if !strings.Contains(server, key) {
			t.Fatalf("server conf missing must-match %q:\n%s", key, server)
		}
		if !strings.Contains(client, key) {
			t.Fatalf("client conf missing must-match %q:\n%s", key, client)
		}
	}

	// One-sided: client only. The server is the responder and these are
	// initiator/local behaviour.
	for _, key := range []string{
		"ContentPaddingAddition = 10-100",
		"RekeyAfterTime = 100-120",
		"RekeyTimeout = 3-7",
		"RejectAfterTime = 150-180",
		"KeepaliveTimeout = 5-15",
		"MaxHandshakeAttempts = 15-20",
	} {
		if !strings.Contains(client, key) {
			t.Fatalf("client conf missing %q:\n%s", key, client)
		}
		if strings.Contains(server, key) {
			t.Fatalf("server conf must not carry client-side %q:\n%s", key, server)
		}
	}

	if !strings.Contains(client, "PersistentKeepalive = 25-35") {
		t.Fatalf("profile keepalive range must win over the server default:\n%s", client)
	}
}

func TestRenderClient_KeepaliveFallsBackToServerDefault(t *testing.T) {
	p := testProfile() // no profile-level PersistentKeepalive
	if got := renderTestClient(t, p); !strings.Contains(got, "PersistentKeepalive = 25") {
		t.Fatalf("expected the server-wide keepalive:\n%s", got)
	}
	out, err := RenderClient(ClientRenderArgs{
		Profile: p, Client: testClient(), AllowedIPs: "0.0.0.0/0",
		Endpoint: "vpn.example:51820", KeepaliveSecs: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "PersistentKeepalive") {
		t.Fatalf("keepalive 0 must omit the key entirely:\n%s", out)
	}
}

func TestRenderConfs_NeverEmitDeadAWG15Keys(t *testing.T) {
	// Itime and J1-J3 exist in no shipping implementation, and amneziawg-tools
	// aborts the interface on any unrecognised key.
	p := testProfile()
	for _, conf := range []string{renderTestProfile(t, p), renderTestClient(t, p)} {
		for _, key := range []string{"Itime", "J1", "J2", "J3"} {
			if strings.Contains(conf, key) {
				t.Fatalf("conf must not emit dead key %q:\n%s", key, conf)
			}
		}
	}
}
