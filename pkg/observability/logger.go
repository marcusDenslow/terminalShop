package observability

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strings"

	"terminalShop/api/middleware"
)

// countingHandler wraps a slog.Handler and increments the log_messages_total
// counter for every record before delegating to the inner handler. This is
// what gives the dashboard a per-level log rate without forcing call sites
// to thread metrics manually.
type countingHandler struct {
	inner slog.Handler
}

func (h *countingHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *countingHandler) Handle(ctx context.Context, r slog.Record) error {
	middleware.RecordLogLevel(strings.ToLower(r.Level.String()))
	return h.inner.Handle(ctx, r)
}

func (h *countingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &countingHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *countingHandler) WithGroup(name string) slog.Handler {
	return &countingHandler{inner: h.inner.WithGroup(name)}
}

// InitLogger installs a slog-based default logger with a level-counting wrapper
// and bridges the stdlib `log` package through slog so existing log.Printf
// calls are also counted (as INFO).
func InitLogger() {
	level := slog.LevelInfo
	if env := strings.ToLower(os.Getenv("LOG_LEVEL")); env != "" {
		switch env {
		case "debug":
			level = slog.LevelDebug
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}

	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	handler := &countingHandler{inner: base}
	slog.SetDefault(slog.New(handler))

	// Route the stdlib log package through slog at INFO so legacy log.Printf
	// calls show up in structured output and bump the counter.
	slog.SetLogLoggerLevel(slog.LevelInfo)
	log.SetFlags(0)
}
