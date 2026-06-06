package awg

import (
	"fmt"
	"io"
	"os/exec"
)

type Runner struct {
	AWGBin      string
	AWGQuickBin string
	Iface       string
}

func (r Runner) Up() error   { return run(r.AWGQuickBin, "up", r.Iface) }
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
