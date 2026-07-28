package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var ErrInvalidNetworkConfig = errors.New("invalid network configuration")

type EnvironmentError struct {
	Field string
	Value string
	Rule  string
}

func (e *EnvironmentError) Error() string {
	return fmt.Sprintf("%s violates %s", e.Field, e.Rule)
}

func (e *EnvironmentError) Is(target error) bool {
	return target == ErrInvalidNetworkConfig
}

type Config struct {
	BindAddr string
	HTTPPort int

	Interface      string
	WGPath         string
	WGHost         string
	PortRangeStart int
	PortRangeEnd   int
	MTU            int
	DNS            string

	Subnet       string
	AllowedIPs   string
	PersistentKA int
	Password     string

	EgressIface string

	AWGBin      string
	AWGQuickBin string

	YookassaShopID    string
	YookassaSecretKey string
	YookassaVatCode   int
	PublicURL         string

	// PaymentContact — короткая инструкция для ручной оплаты (напр. Telegram
	// @handle), показывается плательщику в кабинете, пока не подключена ЮKassa.
	PaymentContact string

	// BillingMinSharePct — при дележе по трафику каждый плательщик платит не
	// меньше этого процента от равной доли (env BILLING_MIN_SHARE_PCT, 0..100).
	BillingMinSharePct int
}

func Load() (Config, error) {
	httpPort, err := networkPort("PORT", 51821)
	if err != nil {
		return Config{}, err
	}
	portRangeStart, err := networkPort("WG_PORT_RANGE_START", 51820)
	if err != nil {
		return Config{}, err
	}
	portRangeEnd, err := networkPort("WG_PORT_RANGE_END", 51859)
	if err != nil {
		return Config{}, err
	}
	if portRangeStart > portRangeEnd {
		return Config{}, &EnvironmentError{Field: "WG_PORT_RANGE", Value: fmt.Sprintf("%d..%d", portRangeStart, portRangeEnd), Rule: "start_not_after_end"}
	}
	if portRangeEnd-portRangeStart+1 > 256 {
		return Config{}, &EnvironmentError{Field: "WG_PORT_RANGE", Value: fmt.Sprintf("%d..%d", portRangeStart, portRangeEnd), Rule: "capacity"}
	}
	wgHost, err := networkHost("WG_HOST")
	if err != nil {
		return Config{}, err
	}
	return Config{
		BindAddr:           env("WEBUI_HOST", "0.0.0.0"),
		HTTPPort:           httpPort,
		Interface:          env("WG_INTERFACE", "awg0"),
		WGPath:             env("WG_PATH", "/etc/amnezia/amneziawg"),
		WGHost:             wgHost,
		PortRangeStart:     portRangeStart,
		PortRangeEnd:       portRangeEnd,
		MTU:                envInt("WG_MTU", 0),
		DNS:                env("WG_DEFAULT_DNS", "1.1.1.1"),
		Subnet:             env("WG_DEFAULT_ADDRESS", "10.8.0.x"),
		// AmneziaVPN client unlocks its split-tunnel UI only when AllowedIPs is
		// full-tunnel: both 0.0.0.0/0 and ::/0 (space after comma matters for
		// older clients that split on ", "). Keep that default. IPv6 blackholes
		// are mitigated with MTU headroom / host v6 — not by stripping ::/0.
		AllowedIPs:         env("WG_ALLOWED_IPS", "0.0.0.0/0, ::/0"),
		PersistentKA:       envInt("WG_PERSISTENT_KEEPALIVE", 0),
		Password:           env("PASSWORD", ""),
		EgressIface:        env("WG_EGRESS_IFACE", detectEgressIface()),
		AWGBin:             env("AWG_BIN", "awg"),
		AWGQuickBin:        env("AWG_QUICK_BIN", "awg-quick"),
		YookassaShopID:     env("YOOKASSA_SHOP_ID", ""),
		YookassaSecretKey:  env("YOOKASSA_SECRET_KEY", ""),
		YookassaVatCode:    envInt("YOOKASSA_VAT_CODE", 1),
		PublicURL:          env("PUBLIC_URL", ""),
		PaymentContact:     strings.TrimSpace(env("PAYMENT_CONTACT", "")),
		BillingMinSharePct: envIntClamp("BILLING_MIN_SHARE_PCT", 25, 0, 100),
	}, nil
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

// detectEgressIface parses `ip route show default` to find the egress NIC. If
// it's wrong, the MASQUERADE rule in awg-quick PostUp silently no-ops and VPN
// clients get a handshake but no internet — far more painful to diagnose than
// a missing env var. Falls back to "eth0" so a manual override is always
// possible via WG_EGRESS_IFACE.
func detectEgressIface() string {
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err != nil {
		return "eth0"
	}
	// Sample line: "default via 10.0.0.1 dev eth0 proto dhcp src 10.0.0.42 metric 100"
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	return "eth0"
}

func envInt(k string, def int) int {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func networkPort(field string, def int) (int, error) {
	raw, ok := os.LookupEnv(field)
	if !ok || strings.TrimSpace(raw) == "" {
		return def, nil
	}
	port, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, &EnvironmentError{Field: field, Value: raw, Rule: "integer"}
	}
	if port < 1 || port > 65535 {
		return 0, &EnvironmentError{Field: field, Value: raw, Rule: "port_range"}
	}
	return port, nil
}

func networkHost(field string) (string, error) {
	raw := env(field, "")
	host := strings.TrimSpace(raw)
	if addr, err := netip.ParseAddr(host); err == nil && addr.Is4() {
		return host, nil
	}
	if validHostname(host) {
		return host, nil
	}
	return "", &EnvironmentError{Field: field, Value: raw, Rule: "hostname_or_ipv4"}
}

func validHostname(host string) bool {
	if host == "" || len(host) > 253 || strings.ContainsAny(host, ":/") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-') {
				return false
			}
		}
	}
	return true
}

// envIntClamp returns envInt(k) bounded to [min, max]; falls back to def when
// unset or unparseable.
func envIntClamp(k string, def, min, max int) int {
	v := envInt(k, def)
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
