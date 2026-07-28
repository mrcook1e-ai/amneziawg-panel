package awg

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

const maxCommandOutput = 1024

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
			slog.Error("AWG runner failed", slog.String("component", "awg"), slog.String("operation", "runner_up"), slog.String("interface", r.Iface), slog.Int("attempt", i+1), slog.Any("error", err))
			return err
		}
		last = err
		slog.Warn("AWG runner retrying", slog.String("component", "awg"), slog.String("operation", "runner_up"), slog.String("interface", r.Iface), slog.Int("attempt", i+1), slog.Any("error", err))
		// Clean any half-attached device the failed attempt left behind so
		// the next try starts from a known-empty namespace state.
		if err := run(r.AWGQuickBin, "down", r.Iface); err != nil {
			slog.Warn("AWG runner cleanup failed", slog.String("component", "awg"), slog.String("operation", "runner_cleanup"), slog.String("interface", r.Iface), slog.Any("error", err))
		}
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
	err := fmt.Errorf("awg-quick up gave up after %d tries: %w", attempts, last)
	slog.Error("AWG runner retries exhausted", slog.String("component", "awg"), slog.String("operation", "runner_up"), slog.String("interface", r.Iface), slog.Int("attempts", attempts), slog.Any("error", err))
	return err
}

func (r Runner) Down() error { return run(r.AWGQuickBin, "down", r.Iface) }

// SyncConf is equivalent to: awg syncconf <iface> <(awg-quick strip <iface>)
// without invoking a shell.
func (r Runner) SyncConf() error {
	stripped, err := commandOutput(r.AWGQuickBin, "strip", r.Iface)
	if err != nil {
		err = fmt.Errorf("awg-quick strip: %w", err)
		slog.Error("AWG configuration strip failed", slog.String("component", "awg"), slog.String("operation", "strip_config"), slog.String("interface", r.Iface), slog.Any("error", err))
		return err
	}
	// Write stripped conf to a real temp file. Passing /dev/stdin works on
	// some hosts but is fragile under non-TTY exec and over-triggers our
	// error redaction ("stdin" substring).
	tmp, err := os.CreateTemp("", "awg-syncconf-*.conf")
	if err != nil {
		return fmt.Errorf("awg syncconf temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(stripped); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("awg syncconf temp write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("awg syncconf temp close: %w", err)
	}
	if out, err := commandCombinedOutput(r.AWGBin, "syncconf", r.Iface, tmpName); err != nil {
		err = commandError(r.AWGBin, []string{"syncconf", r.Iface}, err, out)
		slog.Error("AWG configuration sync failed", slog.String("component", "awg"), slog.String("operation", "sync_config"), slog.String("interface", r.Iface), slog.Any("error", err))
		return err
	}
	return nil
}

func run(bin string, args ...string) error {
	out, err := commandCombinedOutput(bin, args...)
	if err != nil {
		return commandError(bin, args, err, out)
	}
	return nil
}

// commandCombinedOutput runs bin and captures stdout+stderr.
//
// CRITICAL: do NOT use exec.Cmd.CombinedOutput()/pipes here for awg-quick.
// awg-quick spawns long-lived amneziawg-go which inherits the pipe write ends.
// Go's Wait then blocks forever draining pipes that never see EOF — the panel
// never finishes Start(), holds Manager.mu, and every /api/subscribers call
// hangs. Writing to real *os.File descriptors avoids that: Wait returns when
// the direct child exits even if grandchildren still hold the same file.
func commandCombinedOutput(bin string, args ...string) ([]byte, error) {
	f, err := os.CreateTemp("", "awg-cmd-*.log")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	defer func() { _ = os.Remove(path) }()

	cmd := exec.Command(bin, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	runErr := cmd.Run()
	// Rewind and read whatever the child wrote before we close/remove.
	if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
		_ = f.Close()
		if runErr != nil {
			return nil, runErr
		}
		return nil, seekErr
	}
	out, readErr := io.ReadAll(f)
	_ = f.Close()
	if runErr != nil {
		return out, runErr
	}
	return out, readErr
}

func commandOutput(bin string, args ...string) ([]byte, error) {
	// strip is short-lived and does not spawn daemons; file-backed capture is
	// still safer and keeps one code path for redaction.
	return commandCombinedOutput(bin, args...)
}

func commandError(bin string, args []string, err error, out []byte) error {
	safeArgs := sanitizeCommandArgs(args)
	if safeOutput := sanitizeCommandOutput(out); safeOutput != "" {
		return fmt.Errorf("%s %v: %w: %s", bin, safeArgs, err, safeOutput)
	}
	return fmt.Errorf("%s %v: %w", bin, safeArgs, err)
}

func sanitizeCommandOutput(out []byte) string {
	var safe []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if containsSensitiveCommandData(line) {
			safe = append(safe, "[sensitive command output redacted]")
			continue
		}
		safe = append(safe, line)
	}
	output := strings.Join(safe, "\n")
	if len(output) > maxCommandOutput {
		return output[:maxCommandOutput] + " [truncated]"
	}
	return output
}

func sanitizeCommandArgs(args []string) []string {
	safe := make([]string, len(args))
	for i, arg := range args {
		if containsSensitiveCommandData(arg) {
			safe[i] = "[sensitive command argument redacted]"
			continue
		}
		safe[i] = arg
	}
	return safe
}

func containsSensitiveCommandData(value string) bool {
	lower := strings.ToLower(value)
	// Match credential-bearing tokens only. Broad substrings like "config" or
	// "stdin" used to wipe every awg-quick/syncconf failure message in logs
	// ("Unable to modify interface", path names, usage text).
	sensitiveMarkers := []string{
		"privatekey",
		"private_key",
		"presharedkey",
		"preshared_key",
		"publickey",
		"public_key",
		"password",
		"secret",
		"token=",
		"authorization",
		"begin private",
		"begin openssh",
	}
	for _, marker := range sensitiveMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	// Standalone "key =" wireguard conf lines.
	if strings.Contains(lower, "key =") || strings.Contains(lower, "key=") {
		return true
	}
	return false
}
