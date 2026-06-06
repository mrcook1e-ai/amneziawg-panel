package awg

import (
	"bytes"
	"text/template"
	"time"
)

type Server struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	Address    string `json:"address"`

	Jc   int `json:"jc"`
	Jmin int `json:"jmin"`
	Jmax int `json:"jmax"`
	S1   int `json:"s1"`
	S2   int `json:"s2"`
	H1   string `json:"h1"`
	H2   string `json:"h2"`
	H3   string `json:"h3"`
	H4   string `json:"h4"`
}

type Client struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Address      string    `json:"address"`
	PrivateKey   string    `json:"privateKey"`
	PublicKey    string    `json:"publicKey"`
	PreSharedKey string    `json:"preSharedKey"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	// User-editable metadata.
	Notes     string     `json:"notes,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`

	// Per-client overrides — empty/zero means "fall back to server default".
	DNSOverride        string `json:"dnsOverride,omitempty"`
	AllowedIPsOverride string `json:"allowedIPsOverride,omitempty"`
	MTUOverride        int    `json:"mtuOverride,omitempty"`

	// Lifetime counters — survive interface restarts (collector reconciles
	// when the kernel counter resets to zero).
	TotalRx           uint64     `json:"totalRx,omitempty"`
	TotalTx           uint64     `json:"totalTx,omitempty"`
	LastHandshakeAt   *time.Time `json:"lastHandshakeAt,omitempty"`
}

type Config struct {
	Server  Server             `json:"server"`
	Clients map[string]*Client `json:"clients"`
}

var serverTmpl = template.Must(template.New("server").Parse(`# Managed by amneziawg-panel. Do not edit by hand.

[Interface]
PrivateKey = {{.Server.PrivateKey}}
Address = {{.Server.Address}}/24
ListenPort = {{.Port}}
PostUp = iptables -I FORWARD -i %i -j ACCEPT; iptables -I FORWARD -o %i -j ACCEPT; iptables -t nat -A POSTROUTING -s {{.SubnetCIDR}} -o {{.Egress}} -j MASQUERADE
PostDown = iptables -D FORWARD -i %i -j ACCEPT; iptables -D FORWARD -o %i -j ACCEPT; iptables -t nat -D POSTROUTING -s {{.SubnetCIDR}} -o {{.Egress}} -j MASQUERADE
Jc = {{.Server.Jc}}
Jmin = {{.Server.Jmin}}
Jmax = {{.Server.Jmax}}
S1 = {{.Server.S1}}
S2 = {{.Server.S2}}
H1 = {{.Server.H1}}
H2 = {{.Server.H2}}
H3 = {{.Server.H3}}
H4 = {{.Server.H4}}
{{range .Peers}}
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
{{end}}Jc = {{.Server.Jc}}
Jmin = {{.Server.Jmin}}
Jmax = {{.Server.Jmax}}
S1 = {{.Server.S1}}
S2 = {{.Server.S2}}
H1 = {{.Server.H1}}
H2 = {{.Server.H2}}
H3 = {{.Server.H3}}
H4 = {{.Server.H4}}

[Peer]
PublicKey = {{.Server.PublicKey}}
{{if .Client.PreSharedKey}}PresharedKey = {{.Client.PreSharedKey}}
{{end}}AllowedIPs = {{.AllowedIPs}}
{{if .Keepalive}}PersistentKeepalive = {{.Keepalive}}
{{end}}Endpoint = {{.Endpoint}}
`))

type ServerRenderArgs struct {
	Config     *Config
	Port       int
	SubnetCIDR string
	Egress     string
}

func RenderServer(a ServerRenderArgs) ([]byte, error) {
	peers := make([]*Client, 0, len(a.Config.Clients))
	for _, cl := range a.Config.Clients {
		if cl.Enabled {
			peers = append(peers, cl)
		}
	}
	var buf bytes.Buffer
	err := serverTmpl.Execute(&buf, struct {
		Server     *Server
		Port       int
		Peers      []*Client
		SubnetCIDR string
		Egress     string
	}{&a.Config.Server, a.Port, peers, a.SubnetCIDR, a.Egress})
	return buf.Bytes(), err
}

type ClientRenderArgs struct {
	Server     *Server
	Client     *Client
	DNS        string
	MTU        int
	AllowedIPs string
	Endpoint   string
	Keepalive  int
}

func RenderClient(a ClientRenderArgs) ([]byte, error) {
	// Apply per-client overrides on top of server defaults.
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
	var buf bytes.Buffer
	err := clientTmpl.Execute(&buf, a)
	return buf.Bytes(), err
}
