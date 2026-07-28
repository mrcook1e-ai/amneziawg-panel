package awg

import (
	"bytes"
	"fmt"
	"log/slog"
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
	strip := exec.Command(r.AWGQuickBin, "strip", r.Iface)
	stripped, err := strip.Output()
	if err != nil {
		err = fmt.Errorf("awg-quick strip: %w", err)
		slog.Error("AWG configuration strip failed", slog.String("component", "awg"), slog.String("operation", "strip_config"), slog.String("interface", r.Iface), slog.Any("error", err))
		return err
	}
	sync := exec.Command(r.AWGBin, "syncconf", r.Iface, "/dev/stdin")
	sync.Stdin = bytes.NewReader(stripped)
	if out, err := sync.CombinedOutput(); err != nil {
		err = commandError(r.AWGBin, []string{"syncconf", r.Iface}, err, out)
		slog.Error("AWG configuration sync failed", slog.String("component", "awg"), slog.String("operation", "sync_config"), slog.String("interface", r.Iface), slog.Any("error", err))
		return err
	}
	return nil
}

func run(bin string, args ...string) error {
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return commandError(bin, args, err, out)
	}
	return nil
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
	return strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "authorization") || strings.Contains(lower, "config") || strings.Contains(lower, "stdin")
}
