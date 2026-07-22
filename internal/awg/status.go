package awg

import (
	"context"
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
	return parseDump(out, false), nil
}

// ShowAllDump reads every interface through one UAPI command. The "all" dump
// prefixes every row with the interface name, unlike a single-interface dump.
func ShowAllDump(ctx context.Context, bin string) (map[string]PeerStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "show", "all", "dump").Output()
	if err != nil {
		return nil, err
	}
	return parseDump(out, true), nil
}

func parseDump(out []byte, all bool) map[string]PeerStatus {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	res := make(map[string]PeerStatus, len(lines))
	for i, line := range lines {
		if !all && i == 0 {
			continue
		}
		f := strings.Split(line, "\t")
		if all {
			// Interface rows contain the AWG obfuscation fields; peer rows have
			// exactly the interface prefix plus the regular eight peer fields.
			if len(f) != 9 {
				continue
			}
			f = f[1:]
		}
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
	return res
}
