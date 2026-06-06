package awg

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type Runner struct {
	AWGBin      string
	AWGQuickBin string
	Iface       string
}

// Up brings the interface up via awg-quick. In userspace mode (no kernel
// module) there's a known race: awg-quick spawns amneziawg-go and then
// immediately calls `awg setconf` — the UAPI socket sometimes isn't bound
// yet, which surfaces as "Unable to modify interface: Invalid argument" and
// triggers awg-quick to roll the device back. Without retry the panel would
// crash on Start and bounce until timings happened to line up.
func (r Runner) Up() error {
	const attempts = 5
	var last error
	for i := 0; i < attempts; i++ {
		err := run(r.AWGQuickBin, "up", r.Iface)
		if err == nil {
			return nil
		}
		// Only retry the userspace race, not e.g. "Address already in use".
		if !strings.Contains(err.Error(), "Unable to modify interface") {
			return err
		}
		last = err
		// Clean any half-attached device the failed attempt left behind so
		// the next try starts from a known-empty namespace state.
		_ = run(r.AWGQuickBin, "down", r.Iface)
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
	return fmt.Errorf("awg-quick up gave up after %d tries: %w", attempts, last)
}

func (r Runner) Down() error { _ = run(r.AWGQuickBin, "down", r.Iface); return nil }

// SyncConf is equivalent to: awg syncconf <iface> <(awg-quick strip <iface>)
// without invoking a shell.
func (r Runner) SyncConf() error {
	strip := exec.Command(r.AWGQuickBin, "strip", r.Iface)
	stripped, err := strip.Output()
	if err != nil {
		return fmt.Errorf("awg-quick strip: %w", err)
	}
	sync := exec.Command(r.AWGBin, "syncconf", r.Iface, "/dev/stdin")
	stdin, err := sync.StdinPipe()
	if err != nil {
		return err
	}
	if err := sync.Start(); err != nil {
		return err
	}
	if _, err := io.Copy(stdin, bytesReader(stripped)); err != nil {
		stdin.Close()
		return err
	}
	stdin.Close()
	return sync.Wait()
}

func run(bin string, args ...string) error {
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v: %w: %s", bin, args, err, string(out))
	}
	return nil
}

// avoid an extra import line; tiny helper
func bytesReader(b []byte) *byteReader { return &byteReader{b: b} }

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
