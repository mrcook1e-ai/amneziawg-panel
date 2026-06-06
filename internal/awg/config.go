package awg

import (
	"bytes"
	"text/template"
	"time"
)

type Client struct {
	ID           string    `json:"id"`
	ProfileID    string    `json:"profileId"`
	Name         string    `json:"name"`
	Address      string    `json:"address"`
	PrivateKey   string    `json:"privateKey"`
	PublicKey    string    `json:"publicKey"`
	PreSharedKey string    `json:"preSharedKey"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`

	Notes     string     `json:"notes,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`

	DNSOverride        string `json:"dnsOverride,omitempty"`
	AllowedIPsOverride string `json:"allowedIPsOverride,omitempty"`
	MTUOverride        int    `json:"mtuOverride,omitempty"`

	TotalRx         uint64     `json:"totalRx,omitempty"`
	TotalTx         uint64     `json:"totalTx,omitempty"`
	LastHandshakeAt *time.Time `json:"lastHandshakeAt,omitempty"`
}

const SchemaVersion = 2

type Config struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Profiles      map[string]*Profile `json:"profiles"`
	Clients       map[string]*Client  `json:"clients"`
}

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
H1 = {{.Profile.H1}}
H2 = {{.Profile.H2}}
H3 = {{.Profile.H3}}
H4 = {{.Profile.H4}}
{{if .Profile.I1}}I1 = {{.Profile.I1}}
{{end}}{{if .Profile.I2}}I2 = {{.Profile.I2}}
{{end}}{{if .Profile.I3}}I3 = {{.Profile.I3}}
{{end}}{{if .Profile.I4}}I4 = {{.Profile.I4}}
{{end}}{{if .Profile.I5}}I5 = {{.Profile.I5}}
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
H1 = {{.Profile.H1}}
H2 = {{.Profile.H2}}
H3 = {{.Profile.H3}}
H4 = {{.Profile.H4}}
{{if .Profile.I1}}I1 = {{.Profile.I1}}
{{end}}{{if .Profile.I2}}I2 = {{.Profile.I2}}
{{end}}{{if .Profile.I3}}I3 = {{.Profile.I3}}
{{end}}{{if .Profile.I4}}I4 = {{.Profile.I4}}
{{end}}{{if .Profile.I5}}I5 = {{.Profile.I5}}
{{end}}
[Peer]
PublicKey = {{.Profile.PublicKey}}
{{if .Client.PreSharedKey}}PresharedKey = {{.Client.PreSharedKey}}
{{end}}AllowedIPs = {{.AllowedIPs}}
{{if .Keepalive}}PersistentKeepalive = {{.Keepalive}}
{{end}}Endpoint = {{.Endpoint}}
`))

type ProfileRenderArgs struct {
	Profile    *Profile
	Peers      []*Client
	SubnetCIDR string
	Egress     string
}

func RenderProfile(a ProfileRenderArgs) ([]byte, error) {
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
	Keepalive  int
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
	var buf bytes.Buffer
	err := clientTmpl.Execute(&buf, a)
	return buf.Bytes(), err
}
