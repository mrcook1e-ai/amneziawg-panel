package awg

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type PeerStatus struct {
	PublicKey       string
	Endpoint        string
	LatestHandshake *time.Time
	RxBytes         uint64
	TxBytes         uint64
	Keepalive       string
}

func ShowDump(bin, iface string) (map[string]PeerStatus, error) {
	out, err := exec.Command(bin, "show", iface, "dump").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	res := make(map[string]PeerStatus, len(lines))
	for i, line := range lines {
		if i == 0 {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 8 {
			continue
		}
		ps := PeerStatus{
			PublicKey: f[0],
			Endpoint:  f[2],
			Keepalive: f[7],
		}
		if ts, err := strconv.ParseInt(f[4], 10, 64); err == nil && ts > 0 {
			t := time.Unix(ts, 0)
			ps.LatestHandshake = &t
		}
		ps.RxBytes, _ = strconv.ParseUint(f[5], 10, 64)
		ps.TxBytes, _ = strconv.ParseUint(f[6], 10, 64)
		res[ps.PublicKey] = ps
	}
	return res, nil
}
