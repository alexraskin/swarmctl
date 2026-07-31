package logger

import (
	"log/slog"
	"os"
)

// New returns a JSON logger writing to stdout at level. Delivering logs
// somewhere is the log collector's job, not swarmctl's; alerts go through
// internal/notify instead.
func New(level slog.Level, env string, serviceName string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})

	l := slog.New(handler)
	l = l.With("env", env)
	l = l.With("service", serviceName)

	return l
}

func SetDefault(l *slog.Logger) { slog.SetDefault(l) }

func RequestGroup(l *slog.Logger) *slog.Logger { return l.WithGroup("request") }
