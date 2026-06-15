package middle

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/alexraskin/swarmctl/internal/metrics"
)

func AuthMiddleware(expectedToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
				metrics.IncrementAuthFailures()
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
