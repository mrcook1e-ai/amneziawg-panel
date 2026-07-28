package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNew_emitsJSONInfoWhenConfigEmpty(t *testing.T) {
	// Given
	var output bytes.Buffer

	// When
	logger, err := New(&output, Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	logger.Debug("hidden")
	logger.Info("visible")

	// Then
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("log lines = %d, want 1: %q", len(lines), output.String())
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("decode JSON log = %v", err)
	}
	if got := record["level"]; got != "INFO" {
		t.Errorf("level = %v, want INFO", got)
	}
	if got := record["msg"]; got != "visible" {
		t.Errorf("message = %v, want visible", got)
	}
}

func TestNew_emitsTextWhenFormatText(t *testing.T) {
	// Given
	var output bytes.Buffer
	logger, err := New(&output, Config{Format: "text"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// When
	logger.Info("visible")

	// Then
	line := strings.TrimSpace(output.String())
	if strings.HasPrefix(line, "{") {
		t.Errorf("text log unexpectedly starts as JSON: %q", line)
	}
	if !strings.Contains(line, "level=INFO") {
		t.Errorf("text log missing level: %q", line)
	}
	if !strings.Contains(line, "msg=visible") {
		t.Errorf("text log missing message: %q", line)
	}
}

func TestNew_filtersRecordsBelowConfiguredLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  []string
	}{
		{name: "debug", level: "debug", want: []string{"debug", "info", "warn", "error"}},
		{name: "info", level: "info", want: []string{"info", "warn", "error"}},
		{name: "warn", level: "warn", want: []string{"warn", "error"}},
		{name: "error", level: "error", want: []string{"error"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var output bytes.Buffer
			logger, err := New(&output, Config{Level: test.level})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			// When
			logger.Debug("debug")
			logger.Info("info")
			logger.Warn("warn")
			logger.Error("error")

			// Then
			var messages []string
			for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
				var record struct {
					Message string `json:"msg"`
				}
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatalf("decode JSON log = %v", err)
				}
				messages = append(messages, record.Message)
			}
			if got := strings.Join(messages, ","); got != strings.Join(test.want, ",") {
				t.Errorf("messages = %q, want %q", got, strings.Join(test.want, ","))
			}
		})
	}
}

func TestNew_returnsErrorForInvalidConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		want       string
		wantAbsent string
	}{
		{name: "format", config: Config{Format: "xml"}, want: "log format", wantAbsent: "xml"},
		{name: "level", config: Config{Level: "trace"}, want: "log level", wantAbsent: "trace"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var output bytes.Buffer

			// When
			_, err := New(&output, test.config)

			// Then
			if err == nil {
				t.Fatal("New() error = nil, want invalid configuration error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("New() error = %q, want %q", err, test.want)
			}
			if strings.Contains(err.Error(), test.wantAbsent) {
				t.Errorf("New() error = %q, must not echo invalid value", err)
			}
		})
	}
}
