package server

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AuthToken                  string
	CloudflareTunnelID         string
	CloudflareAPIToken         string
	CloudflareAccountID        string
	Environment                string
	NotificationURLs           []string
	ServiceRemovalDelayMinutes int
	DeleteDNSOnRemoval         bool
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		slog.Error("No .env file found or error reading .env:", "error", err)
	}

	// Default to 30 minutes if not set
	removalDelay := 30
	if delayStr := os.Getenv("SERVICE_REMOVAL_DELAY_MINUTES"); delayStr != "" {
		if parsed, err := strconv.Atoi(delayStr); err == nil {
			removalDelay = parsed
		}
	}

	// Default to false if not set
	deleteDNS := false
	if dnsStr := os.Getenv("DELETE_DNS_ON_REMOVAL"); dnsStr != "" {
		deleteDNS = strings.ToLower(dnsStr) == "true"
	}

	// Notifications are optional: no URLs means no alerts, not a failed start.
	notificationURLs := parseNotificationURLs(getOptionalSecretOrEnv("NOTIFICATION_URLS"))

	return &Config{
		AuthToken:                  getSecretOrEnv("AUTH_TOKEN"),
		CloudflareTunnelID:         getSecretOrEnv("CLOUDFLARE_TUNNEL_ID"),
		CloudflareAPIToken:         getSecretOrEnv("CLOUDFLARE_API_TOKEN"),
		CloudflareAccountID:        getSecretOrEnv("CLOUDFLARE_ACCOUNT_ID"),
		Environment:                getSecretOrEnv("ENVIRONMENT"),
		NotificationURLs:           notificationURLs,
		ServiceRemovalDelayMinutes: removalDelay,
		DeleteDNSOnRemoval:         deleteDNS,
	}
}

// parseNotificationURLs splits a shoutrrr URL list on commas and newlines, so a
// Docker secret file can hold one URL per line and an env var can hold a
// comma-separated list. Empty entries are dropped.
func parseNotificationURLs(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})

	urls := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	return urls
}

// getOptionalSecretOrEnv reads key with the same Docker-secret handling as
// getSecretOrEnv, but returns "" for an unset value instead of exiting. Use it
// for anything swarmctl can run without.
func getOptionalSecretOrEnv(key string) string {
	value := os.Getenv(key)

	if strings.HasPrefix(value, "/") {
		if _, err := os.Stat(value); err == nil {
			data, err := os.ReadFile(value)
			if err != nil {
				slog.Error("Failed to read secret file", "key", key, "path", value, "error", err)
				return ""
			}
			return strings.TrimSpace(string(data))
		}
	}

	return value
}

func getSecretOrEnv(key string) string {
	value := os.Getenv(key)

	if strings.HasPrefix(value, "/") {
		if _, err := os.Stat(value); err == nil {
			data, err := os.ReadFile(value)
			if err != nil {
				slog.Error("Failed to read secret file", "key", key, "path", value, "error", err)
			}
			return strings.TrimSpace(string(data))
		}
	}

	if value == "" {
		slog.Error("Environment variable is not set", "key", key)
		os.Exit(-1)
	}

	return value
}
