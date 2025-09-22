package logger

import (
	"log/slog"
	"os"

	slogdiscord "github.com/betrayy/slog-discord"
)

func New(level slog.Level, webhookURL string, env string, serviceName string) (*slog.Logger, error) {
	opts := []slogdiscord.Option{
		slogdiscord.WithMinLevel(slog.LevelWarn),
		slogdiscord.WithSyncMode(true),
		slogdiscord.WithHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})),
	}

	discordHandler, err := slogdiscord.NewDiscordHandler(webhookURL, opts...)
	if err != nil {
		return nil, err
	}

	l := slog.New(discordHandler)
	l = l.With("env", env)
	l = l.With("service", serviceName)

	return l, nil
}

func SetDefault(l *slog.Logger) { slog.SetDefault(l) }

func RequestGroup(l *slog.Logger) *slog.Logger { return l.WithGroup("request") }
