package server

import (
	"log"
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
}

func NewConfigFromEnv() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found or error reading .env:", err)
	}
	return &Config{
		AuthToken:           getSecretOrEnv("AUTH_TOKEN"),
		CloudflareTunnelID:  getSecretOrEnv("CLOUDFLARE_TUNNEL_ID"),
		CloudflareAPIKey:    getSecretOrEnv("CLOUDFLARE_API_KEY"),
		CloudflareAPIEmail:  getSecretOrEnv("CLOUDFLARE_API_EMAIL"),
		CloudflareAccountID: getSecretOrEnv("CLOUDFLARE_ACCOUNT_ID"),
		PushoverAPIKey:      getSecretOrEnv("PUSHOVER_API_KEY"),
		PushoverRecipient:   getSecretOrEnv("PUSHOVER_RECIPIENT"),
	}
}

func getSecretOrEnv(key string) string {
	value := os.Getenv(key)

	if strings.HasPrefix(value, "/") {
		data, err := os.ReadFile(value)
		if err != nil {
			log.Fatalf("Failed to read secret file for %s: %v", key, err)
		}
		return strings.TrimSpace(string(data))
	}

	if value == "" {
		log.Fatalf("Environment variable %s is not set", key)
	}

	return value
}
