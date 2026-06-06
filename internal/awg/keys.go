package awg

import (
	"bytes"
	"os/exec"
	"strings"
)

type Keys struct{ Bin string }

func (k Keys) GenPrivate() (string, error) { return k.run(nil, "genkey") }
func (k Keys) GenPSK() (string, error)     { return k.run(nil, "genpsk") }

func (k Keys) Public(priv string) (string, error) {
	return k.run([]byte(priv+"\n"), "pubkey")
}

func (k Keys) run(stdin []byte, args ...string) (string, error) {
	cmd := exec.Command(k.Bin, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
