package middle

import (
	"crypto/subtle"
	"net/http"

	"github.com/alexraskin/swarmctl/internal/metrics"
)

func AuthMiddleware(expectedToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if len(token) < 8 || subtle.ConstantTimeCompare([]byte(token[7:]), []byte(expectedToken)) != 1 {
				metrics.IncrementAuthFailures()
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
