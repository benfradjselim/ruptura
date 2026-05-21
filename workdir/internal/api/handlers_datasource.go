package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/scraper"
	"github.com/gorilla/mux"
)

// GET /api/v2/datasources
func (h *Handlers) handleListDatasources(w http.ResponseWriter, r *http.Request) {
	if h.scraper == nil {
		writeJSON(w, http.StatusOK, []scraper.DatasourceStatus{})
		return
	}
	writeJSON(w, http.StatusOK, h.scraper.List())
}

// GET /api/v2/datasources/{id}
func (h *Handlers) handleGetDatasource(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if h.scraper == nil {
		writeError(w, http.StatusNotFound, "scraper not available")
		return
	}
	ds, ok := h.scraper.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}
	writeJSON(w, http.StatusOK, ds)
}

// POST /api/v2/datasources
func (h *Handlers) handleCreateDatasource(w http.ResponseWriter, r *http.Request) {
	if h.scraper == nil {
		writeError(w, http.StatusServiceUnavailable, "scraper not available")
		return
	}
	var cfg scraper.DatasourceConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if cfg.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if err := validateDatasourceURL(cfg.URL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if cfg.Type == "" {
		cfg.Type = scraper.TypeDirect
	}
	if cfg.ID == "" {
		cfg.ID = generateDSID(cfg.URL, cfg.Type)
	}
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = cfg.CreatedAt

	if err := h.scraper.Put(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ds, _ := h.scraper.Get(cfg.ID)
	writeJSON(w, http.StatusCreated, ds)
}

// PUT /api/v2/datasources/{id}
func (h *Handlers) handleUpdateDatasource(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if h.scraper == nil {
		writeError(w, http.StatusServiceUnavailable, "scraper not available")
		return
	}
	var cfg scraper.DatasourceConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cfg.ID = id
	cfg.UpdatedAt = time.Now()

	if err := h.scraper.Put(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ds, _ := h.scraper.Get(id)
	writeJSON(w, http.StatusOK, ds)
}

// DELETE /api/v2/datasources/{id}
func (h *Handlers) handleDeleteDatasource(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if h.scraper == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.scraper.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v2/datasources/{id}/test  — OR  POST /api/v2/datasources/test with body
func (h *Handlers) handleTestDatasource(w http.ResponseWriter, r *http.Request) {
	if h.scraper == nil {
		writeError(w, http.StatusServiceUnavailable, "scraper not available")
		return
	}

	var cfg scraper.DatasourceConfig
	id := mux.Vars(r)["id"]
	if id != "" && id != "test" {
		// test existing datasource by ID
		ds, ok := h.scraper.Get(id)
		if !ok {
			writeError(w, http.StatusNotFound, "datasource not found")
			return
		}
		cfg = ds.DatasourceConfig
	} else {
		// test an ad-hoc config from request body
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := validateDatasourceURL(cfg.URL); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	count, errMsg := h.scraper.Test(cfg)
	result := map[string]interface{}{
		"ok":              errMsg == "",
		"scraped_metrics": count,
	}
	if errMsg != "" {
		result["error"] = errMsg
	}
	writeJSON(w, http.StatusOK, result)
}

// validateDatasourceURL rejects non-http(s) schemes and private/link-local addresses (SSRF prevention).
func validateDatasourceURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	hostname := u.Hostname()
	// Cluster-internal service names are intentional, not SSRF targets.
	if strings.HasSuffix(hostname, ".svc.cluster.local") || strings.HasSuffix(hostname, ".cluster.local") {
		return nil
	}
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		// treat unresolvable as safe — scraper will fail at fetch time
		return nil
	}
	private := []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"127.0.0.0/8", "169.254.0.0/16", "::1/128", "fc00::/7",
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil {
			continue
		}
		for _, cidr := range private {
			_, block, _ := net.ParseCIDR(cidr)
			if block.Contains(ip) {
				return fmt.Errorf("datasource URL resolves to a private or link-local address")
			}
		}
	}
	return nil
}

// generateDSID creates a stable ID from a URL and type.
func generateDSID(rawURL, dsType string) string {
	// strip scheme and normalize
	s := strings.NewReplacer("://", "-", "/", "-", ":", "-", ".", "-").Replace(rawURL)
	s = strings.TrimRight(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	return dsType + "-" + s
}
