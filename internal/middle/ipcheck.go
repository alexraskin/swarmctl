package middle

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

var allowedIPs = []string{}

func UpdateIPList(devMode bool, logger *slog.Logger) {
	resp, err := http.Get("https://api.github.com/meta")
	if err != nil {
		logger.Error("Failed to fetch IPs", "error", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", "error", err)
		return
	}

	var meta struct {
		Actions []string `json:"actions"`
	}

	if err := json.Unmarshal(body, &meta); err != nil {
		logger.Error("Failed to parse IP list", "error", err)
		return
	}

	allowedIPs = meta.Actions

	localIPs := []string{}
	if devMode {
		localIPs = []string{
			"127.0.0.1",
			"::1",
			"localhost",
			"10.",
			"172.16.",
			"172.17.",
			"172.18.",
			"172.19.",
			"172.20.",
			"172.21.",
			"172.22.",
			"172.23.",
			"172.24.",
			"172.25.",
			"172.26.",
			"172.27.",
			"172.28.",
			"172.29.",
			"172.30.",
			"172.31.",
			"192.168.",
		}
		allowedIPs = append(allowedIPs, localIPs...)
	}

	logger.Debug("Updated IP allowlist", "count", len(allowedIPs))
}

func isAllowedIP(ip string) bool {
	for _, allowedIP := range allowedIPs {
		if strings.HasPrefix(ip, allowedIP) {
			return true
		}
	}
	return false
}

func IPCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r.RemoteAddr)
		if !isAllowedIP(ip) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractIP(remoteAddr string) string {
	if strings.HasPrefix(remoteAddr, "[") {
		end := strings.Index(remoteAddr, "]")
		if end > 0 {
			return remoteAddr[1:end]
		}
	}

	if strings.Contains(remoteAddr, ":") {
		parts := strings.Split(remoteAddr, ":")
		return parts[0]
	}

	return remoteAddr
}
