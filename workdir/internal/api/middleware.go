package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ohe/pkg/logger"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	claimsKey contextKey = "claims"
	orgIDKey  contextKey = "org_id"
)

// JWTClaims represents JWT payload
type JWTClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	OrgID    string `json:"org_id,omitempty"`
	jwt.RegisteredClaims
}

// orgIDFromContext returns the org ID from context, defaulting to "default".
func orgIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(orgIDKey).(string); ok && v != "" {
		return v
	}
	return "default"
}

// respondJSON writes a JSON response with the given status code
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Default.Error("encode response", "err", err)
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

// authMiddlewareMu guards the two pluggable auth functions below.
var authMiddlewareMu sync.RWMutex

// apiKeyLookupFn is set by the orchestrator after wiring storage so AuthMiddleware
// can validate API keys without a direct import of the storage package.
var apiKeyLookupFn func(orgID, rawKey string) (*JWTClaims, bool)

// tokenRevokedFn is set by the orchestrator to check the JWT revocation blocklist.
var tokenRevokedFn func(jti string) bool

// SetAPIKeyLookup wires the API key validation function used by AuthMiddleware.
// Must be called before the HTTP server starts.
func SetAPIKeyLookup(fn func(orgID, rawKey string) (*JWTClaims, bool)) {
	authMiddlewareMu.Lock()
	defer authMiddlewareMu.Unlock()
	apiKeyLookupFn = fn
}

// SetTokenRevokedChecker wires the JTI blocklist checker used by AuthMiddleware.
// Must be called before the HTTP server starts.
func SetTokenRevokedChecker(fn func(jti string) bool) {
	authMiddlewareMu.Lock()
	defer authMiddlewareMu.Unlock()
	tokenRevokedFn = fn
}

// AuthMiddleware validates Bearer tokens when auth is enabled.
// It first attempts JWT validation; if the token starts with the "ohe_" prefix
// it is treated as an API key and validated against the key store instead.
func AuthMiddleware(jwtSecret string, enabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auth for health probes, login, and first-run setup (SetupHandler guards itself)
			if r.URL.Path == "/api/v1/health" ||
				r.URL.Path == "/api/v1/health/live" ||
				r.URL.Path == "/api/v1/health/ready" ||
				r.URL.Path == "/api/v1/auth/login" ||
				r.URL.Path == "/api/v1/auth/setup" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid token")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			// API key path: tokens starting with "ohe_" are not JWTs
			if strings.HasPrefix(tokenStr, apiKeyPrefix) {
				authMiddlewareMu.RLock()
				lookup := apiKeyLookupFn
				authMiddlewareMu.RUnlock()

				if lookup == nil {
					respondError(w, http.StatusUnauthorized, "INVALID_TOKEN", "api key authentication not available")
					return
				}
				// API keys are org-scoped; we need the org from the key prefix itself.
				// The lookup function resolves org internally via the key prefix.
				claims, ok := lookup("", tokenStr)
				if !ok {
					respondError(w, http.StatusUnauthorized, "INVALID_TOKEN", "invalid or expired api key")
					return
				}
				ctx := context.WithValue(r.Context(), claimsKey, claims)
				if claims.OrgID != "" {
					ctx = context.WithValue(ctx, orgIDKey, claims.OrgID)
					ctx = logger.WithOrgID(ctx, claims.OrgID)
				}
				ctx = logger.WithUsername(ctx, claims.Username)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// JWT path
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

			// Check revocation blocklist (populated by LogoutHandler)
			authMiddlewareMu.RLock()
			revoked := tokenRevokedFn
			authMiddlewareMu.RUnlock()
			if revoked != nil && claims.ID != "" && revoked(claims.ID) {
				respondError(w, http.StatusUnauthorized, "TOKEN_REVOKED", "token has been revoked")
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			if claims.OrgID != "" {
				ctx = context.WithValue(ctx, orgIDKey, claims.OrgID)
				ctx = logger.WithOrgID(ctx, claims.OrgID)
			}
			ctx = logger.WithUsername(ctx, claims.Username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LoggingMiddleware injects a unique request_id into the context and logs each request.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := newRequestID()
		ctx := logger.WithRequestID(r.Context(), rid)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", rid)

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		logger.Default.InfoCtx(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
