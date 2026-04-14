package api

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const claimsKey contextKey = "claims"

// JWTClaims represents JWT payload
type JWTClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// respondJSON writes a JSON response with the given status code
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// respondError writes a JSON error response
func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, map[string]interface{}{
		"success": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
		"timestamp": time.Now().UTC(),
	})
}

// respondSuccess writes a successful JSON response
func respondSuccess(w http.ResponseWriter, data interface{}) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"data":      data,
		"timestamp": time.Now().UTC(),
	})
}

// AuthMiddleware validates Bearer JWT tokens when auth is enabled
func AuthMiddleware(jwtSecret string, enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth for health and login
			if r.URL.Path == "/api/v1/health" || r.URL.Path == "/api/v1/auth/login" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid token")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				respondError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LoggingMiddleware logs incoming requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

// CORSMiddleware returns a middleware that enforces the CORS allowlist.
// If allowedOrigins is empty, wildcard "*" is used (dev mode only).
// In production, pass the configured origin allowlist.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	wildcard := len(allowedOrigins) == 0

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
				// No CORS header if origin not in allowlist — browser will block
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware adds security response headers
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// RequireRole returns a middleware that enforces the minimum role level.
// Roles: viewer < operator < admin.
// When auth is disabled (no claims in context) all roles are permitted — the
// AuthMiddleware upstream already decided not to enforce authentication.
func RequireRole(role string) func(http.Handler) http.Handler {
	roleLevel := map[string]int{"viewer": 1, "operator": 2, "admin": 3}
	required := roleLevel[role]
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := claimsFromContext(r.Context())
			if !ok {
				// No claims means auth is disabled globally; allow the request.
				next.ServeHTTP(w, r)
				return
			}
			if roleLevel[claims.Role] < required {
				respondError(w, http.StatusForbidden, "FORBIDDEN", "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ipLimiter holds per-IP token-bucket state
type ipLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

var loginLimiter = &ipLimiter{buckets: make(map[string]*bucket)}

// evictStaleBuckets removes buckets inactive for more than 10 minutes.
// Must be called with loginLimiter.mu held.
func evictStaleBuckets() {
	threshold := time.Now().Add(-10 * time.Minute)
	for ip, b := range loginLimiter.buckets {
		if b.lastSeen.Before(threshold) {
			delete(loginLimiter.buckets, ip)
		}
	}
}

// RateLimitLogin allows up to 5 login attempts per minute per IP
func RateLimitLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}
		loginLimiter.mu.Lock()
		b, ok := loginLimiter.buckets[ip]
		if !ok {
			// Evict stale entries on new IP registration to bound memory
			if len(loginLimiter.buckets) > 10000 {
				evictStaleBuckets()
			}
			b = &bucket{tokens: 5, lastSeen: time.Now()}
			loginLimiter.buckets[ip] = b
		}
		// Refill: 5 tokens per minute
		elapsed := time.Since(b.lastSeen).Seconds()
		b.tokens += elapsed * (5.0 / 60.0)
		if b.tokens > 5 {
			b.tokens = 5
		}
		b.lastSeen = time.Now()
		allow := b.tokens >= 1
		if allow {
			b.tokens--
		}
		loginLimiter.mu.Unlock()

		if !allow {
			respondError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many login attempts, try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// claimsFromContext retrieves JWT claims from context
func claimsFromContext(ctx context.Context) (*JWTClaims, bool) {
	c, ok := ctx.Value(claimsKey).(*JWTClaims)
	return c, ok
}
