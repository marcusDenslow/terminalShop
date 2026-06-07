package audit

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestCartRejected(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	CartRejected(42, 25000, 20000)

	raw := strings.TrimSpace(buf.String())
	if raw == "" {
		t.Fatalf("CartRejected: no slog output captured")
	}

	var rec map[string]any
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		t.Fatalf("decode slog record: %v\nraw: %s", err, raw)
	}

	if got, want := rec["event"], "cart_rejected"; got != want {
		t.Errorf("event: want %q, got %v", want, got)
	}
	if n, _ := rec["user_id"].(float64); int(n) != 42 {
		t.Errorf("user_id want 42, got %v", rec["user_id"])
	}
	if n, _ := rec["total_cents"].(float64); int(n) != 25000 {
		t.Errorf("total_cents: want 25000, got %v", rec["total_cents"])
	}
	if n, _ := rec["cap_cents"].(float64); int(n) != 20000 {
		t.Errorf("cap_cents: want 20000, got %v", rec["cap_cents"])
	}
}
