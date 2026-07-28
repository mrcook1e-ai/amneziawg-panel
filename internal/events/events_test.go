package events

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/mrcook1e/amneziawg-panel/internal/db"
)

func TestLogAppendReportsClosedDB(t *testing.T) {
	// Given
	store, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	// When
	New(store).Append(KindClientCreated, "client-1", map[string]string{"token": "secret-token"})

	// Then
	var record map[string]any
	if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
		t.Fatalf("log record = %q, want JSON error record: %v", logs.String(), err)
	}
	if got := record["component"]; got != "events" {
		t.Fatalf("component = %v, want events", got)
	}
	if got := record["operation"]; got != "append" {
		t.Fatalf("operation = %v, want append", got)
	}
	if _, ok := record["error"]; !ok {
		t.Fatal("error field is missing")
	}
	if strings.Contains(logs.String(), "secret-token") {
		t.Fatal("log leaked event payload")
	}
}
