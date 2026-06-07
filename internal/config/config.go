package config

import (
	"os"
	"strconv"
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

	Subnet           string
	AllowedIPs       string
	PersistentKA     int
	Password         string

	EgressIface string

	AWGBin      string
	AWGQuickBin string
}

func Load() Config {
	return Config{
		BindAddr:     env("WEBUI_HOST", "0.0.0.0"),
		HTTPPort:     envInt("PORT", 51821),
		Interface:      env("WG_INTERFACE", "awg0"),
		WGPath:         env("WG_PATH", "/etc/amnezia/amneziawg"),
		WGHost:         env("WG_HOST", ""),
		WGPort:         envInt("WG_PORT", 51820),
		PortRangeStart: envInt("WG_PORT_RANGE_START", 51820),
		PortRangeEnd:   envInt("WG_PORT_RANGE_END", 51829),
		MTU:          envInt("WG_MTU", 0),
		DNS:          env("WG_DEFAULT_DNS", "1.1.1.1"),
		Subnet:       env("WG_DEFAULT_ADDRESS", "10.8.0.x"),
		AllowedIPs:   env("WG_ALLOWED_IPS", "0.0.0.0/0, ::/0"),
		PersistentKA: envInt("WG_PERSISTENT_KEEPALIVE", 0),
		Password:     env("PASSWORD", ""),
		EgressIface:  env("WG_EGRESS_IFACE", "eth0"),
		AWGBin:       env("AWG_BIN", "awg"),
		AWGQuickBin:  env("AWG_QUICK_BIN", "awg-quick"),
	}
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
