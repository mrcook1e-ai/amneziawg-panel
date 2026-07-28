package config

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Config struct {
	BindAddr string
	HTTPPort int

	Interface      string
	WGPath         string
	WGHost         string
	WGPort         int
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

func Load() Config {
	return Config{
		BindAddr:           env("WEBUI_HOST", "0.0.0.0"),
		HTTPPort:           envInt("PORT", 51821),
		Interface:          env("WG_INTERFACE", "awg0"),
		WGPath:             env("WG_PATH", "/etc/amnezia/amneziawg"),
		WGHost:             env("WG_HOST", ""),
		WGPort:             envInt("WG_PORT", 51820),
		PortRangeStart:     envInt("WG_PORT_RANGE_START", 51820),
		PortRangeEnd:       envInt("WG_PORT_RANGE_END", 51859),
		MTU:                envInt("WG_MTU", 0),
		DNS:                env("WG_DEFAULT_DNS", "1.1.1.1"),
		Subnet:             env("WG_DEFAULT_ADDRESS", "10.8.0.x"),
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
	}
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
