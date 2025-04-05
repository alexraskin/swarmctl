package server

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	AuthToken string
}

func NewConfig(authToken string) *Config {
	return &Config{
		AuthToken: authToken,
	}
}

func GetAuthToken() string {
	envValue := os.Getenv("AUTH_TOKEN")

	if strings.HasPrefix(envValue, "/") {
		data, err := os.ReadFile(envValue)
		if err != nil {
			log.Fatalf("Failed to read auth token from secret file: %v", err)
		}
		return strings.TrimSpace(string(data))
	}

	if envValue == "" {
		log.Fatal("AUTH_TOKEN is not set")
	}

	return envValue
}
