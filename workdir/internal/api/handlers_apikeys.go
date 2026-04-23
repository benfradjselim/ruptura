package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/benfradjselim/ohe/pkg/models"
	"github.com/benfradjselim/ohe/pkg/utils"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

const apiKeyPrefix = "ohe_"

// APIKeyListHandler GET /api/v1/api-keys
// Returns all API keys for the caller's org (key hashes are omitted).
func (h *Handlers) APIKeyListHandler(w http.ResponseWriter, r *http.Request) {
	var keys []models.APIKey
	err := h.orgStore(r).ListAPIKeys(func(val []byte) error {
		var k models.APIKey
		if err := json.Unmarshal(val, &k); err != nil {
			return nil // skip corrupt records
		}
		k.KeyHash = "" // never expose hash
		keys = append(keys, k)
		return nil
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	if keys == nil {
		keys = []models.APIKey{}
	}
	respondSuccess(w, keys)
}

// APIKeyCreateHandler POST /api/v1/api-keys
// Generates a new API key. The full plaintext key is returned only once.
func (h *Handlers) APIKeyCreateHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.orgStore(r).CheckAPIKeyQuota(h.orgQuota(r).MaxAPIKeys); err != nil {
		respondError(w, http.StatusPaymentRequired, "QUOTA_EXCEEDED", err.Error())
		return
	}
	var req models.APIKeyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "INVALID_NAME", "name is required")
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	switch req.Role {
	case "viewer", "operator", "admin":
	default:
		respondError(w, http.StatusBadRequest, "INVALID_ROLE", "role must be viewer, operator, or admin")
		return
	}

	// Generate cryptographically random 32-byte secret
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		respondError(w, http.StatusInternalServerError, "KEYGEN_ERROR", "failed to generate key")
		return
	}
	fullKey := apiKeyPrefix + hex.EncodeToString(secret)
	prefix := fullKey[:12] // "ohe_" + 8 hex chars, e.g. "ohe_a1b2c3d4"

	hash, err := bcrypt.GenerateFromPassword([]byte(fullKey), 12)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "HASH_ERROR", err.Error())
		return
	}

	orgID := orgIDFromContext(r.Context())
	now := time.Now().UTC()
	var expiresAt time.Time
	if d, err := parseDuration(req.ExpiresIn); err == nil && d > 0 {
		expiresAt = now.Add(d)
	}

	key := models.APIKey{
		ID:        utils.GenerateID(12),
		OrgID:     orgID,
		Name:      req.Name,
		Role:      req.Role,
		KeyHash:   string(hash),
		Prefix:    prefix,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Active:    true,
	}

	if err := h.orgStore(r).SaveAPIKey(key.ID, key); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	h.audit(r, "create", "api_key", key.ID, key.Name)

	resp := models.APIKeyCreateResponse{
		APIKey:       key,
		PlaintextKey: fullKey,
	}
	resp.KeyHash = "" // do not expose hash in response
	respondSuccess(w, resp)
}

// APIKeyDeleteHandler DELETE /api/v1/api-keys/{id}
func (h *Handlers) APIKeyDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.orgStore(r).DeleteAPIKey(id); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("api key %s not found", id))
		return
	}
	h.audit(r, "delete", "api_key", id, "")
	respondSuccess(w, map[string]string{"deleted": id})
}

// parseDuration parses strings like "30d", "90d", "24h", "7d" into time.Duration.
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// ValidateAPIKey checks a raw API key against stored keys in the given org store.
// Returns (claims, ok) — claims carry the org/role from the key record.
func ValidateAPIKey(orgStore interface {
	LookupAPIKeyByPrefix(prefix string, dest interface{}) error
}, rawKey string) (*JWTClaims, bool) {
	if len(rawKey) < 12 || !strings.HasPrefix(rawKey, apiKeyPrefix) {
		return nil, false
	}
	prefix := rawKey[:12]

	var key models.APIKey
	if err := orgStore.LookupAPIKeyByPrefix(prefix, &key); err != nil || key.ID == "" {
		return nil, false
	}
	if !key.Active {
		return nil, false
	}
	if !key.ExpiresAt.IsZero() && time.Now().After(key.ExpiresAt) {
		return nil, false
	}
	if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(rawKey)); err != nil {
		return nil, false
	}
	return &JWTClaims{
		Username: "apikey:" + key.Name,
		Role:     key.Role,
		OrgID:    key.OrgID,
	}, true
}
