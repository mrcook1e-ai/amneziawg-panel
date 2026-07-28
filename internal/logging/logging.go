// Package logging configures the process-wide structured logger.
package logging

import (
	"errors"
	"io"
	"log/slog"
	"strings"
)

// Config selects the output format and minimum log level.
type Config struct {
	Format string
	Level  string
}

// New returns a logger that writes structured records to output.
func New(output io.Writer, config Config) (*slog.Logger, error) {
	level, err := parseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	options := &slog.HandlerOptions{Level: level}
	switch strings.ToLower(strings.TrimSpace(config.Format)) {
	case "", "json":
		return slog.New(slog.NewJSONHandler(output, options)), nil
	case "text":
		return slog.New(slog.NewTextHandler(output, options)), nil
	default:
		return nil, errors.New("invalid log format")
	}
}

func parseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, errors.New("invalid log level")
	}
}
