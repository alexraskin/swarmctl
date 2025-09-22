package server

import (
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AuthToken           string
	CloudflareTunnelID  string
	CloudflareAPIKey    string
	CloudflareAPIEmail  string
	CloudflareAccountID string
	PushoverAPIKey      string
	PushoverRecipient   string
	Environment         string
	WebhookURL          string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		slog.Error("No .env file found or error reading .env:", "error", err)
	}
	return &Config{
		AuthToken:           getSecretOrEnv("AUTH_TOKEN"),
		CloudflareTunnelID:  getSecretOrEnv("CLOUDFLARE_TUNNEL_ID"),
		CloudflareAPIKey:    getSecretOrEnv("CLOUDFLARE_API_KEY"),
		CloudflareAPIEmail:  getSecretOrEnv("CLOUDFLARE_API_EMAIL"),
		CloudflareAccountID: getSecretOrEnv("CLOUDFLARE_ACCOUNT_ID"),
		PushoverAPIKey:      getSecretOrEnv("PUSHOVER_API_KEY"),
		PushoverRecipient:   getSecretOrEnv("PUSHOVER_RECIPIENT"),
		Environment:         getSecretOrEnv("ENVIROMENT"),
		WebhookURL:          getSecretOrEnv("WEBHOOK_URL"),
	}
}

func getSecretOrEnv(key string) string {
	value := os.Getenv(key)

	if strings.HasPrefix(value, "/") {
		if _, err := os.Stat(value); err == nil {
			data, err := os.ReadFile(value)
			if err != nil {
				slog.Error("Failed to read secret file for %s", "error", err)
			}
			return strings.TrimSpace(string(data))
		}
	}

	if value == "" {
		slog.Error("Environment variable %s is not set", "error", key)
		os.Exit(-1)
	}

	return value
}
