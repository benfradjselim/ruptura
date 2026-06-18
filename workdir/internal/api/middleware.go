package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
)

func (h *Handlers) authMiddleware(next http.Handler) http.Handler {
	if h.apiKey == "" {
		log.Println("WARNING: RUPTURA_API_KEY is not set — server running in UNAUTHENTICATED mode. Set RUPTURA_API_KEY env var.")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Probe endpoints are always public — k8s liveness/readiness carry no auth.
		// /api/v2/health and /api/v2/ready are registered on the root router (not this subrouter)
		// so they never reach this middleware. Auth-login endpoints are also on root router.

		if h.apiKey == "" {
			// Fail-closed: no API key configured means all routes are blocked.
			// This prevents accidental unauthenticated exposure in misconfigured deployments.
			writeError(w, http.StatusServiceUnavailable,
				"RUPTURA_API_KEY is not configured — set it via env or --api-key flag")
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") ||
			subtle.ConstantTimeCompare([]byte(auth[7:]), []byte(h.apiKey)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
