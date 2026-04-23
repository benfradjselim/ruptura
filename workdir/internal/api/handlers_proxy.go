package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/benfradjselim/ohe/pkg/models"
	"github.com/benfradjselim/ohe/pkg/utils"
	"github.com/gorilla/mux"
)

// DataSourceProxyHandler proxies PromQL/HTTP queries to a registered datasource.
// POST /api/v1/datasources/{id}/proxy
// Body: {"query":"...", "start":"...","end":"...","step":15,"type":"query_range"}
func (h *Handlers) DataSourceProxyHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var ds models.DataSource
	if err := h.store.GetDataSource(id, &ds); err != nil {
		respondError(w, http.StatusNotFound, "DS_NOT_FOUND", "datasource not found")
		return
	}
	if !ds.Enabled {
		respondError(w, http.StatusBadRequest, "DS_DISABLED", "datasource is disabled")
		return
	}

	if err := validateDataSourceURL(ds.URL); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_URL", err.Error())
		return
	}

	var req struct {
		Query string `json:"query"`
		Start string `json:"start"`
		End   string `json:"end"`
		Step  int    `json:"step"`
		Type  string `json:"type"` // "query_range" | "query" | "series"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if req.Query == "" {
		respondError(w, http.StatusBadRequest, "MISSING_QUERY", "query is required")
		return
	}

	qType := req.Type
	if qType == "" {
		qType = "query_range"
	}
	step := req.Step
	if step <= 0 {
		step = 15
	}

	// Build upstream URL
	base := strings.TrimRight(ds.URL, "/")
	upstreamURL := fmt.Sprintf("%s/api/v1/%s", base, qType)

	params := url.Values{}
	params.Set("query", req.Query)
	if req.Start != "" {
		params.Set("start", req.Start)
	} else {
		params.Set("start", fmt.Sprintf("%d", time.Now().Add(-1*time.Hour).Unix()))
	}
	if req.End != "" {
		params.Set("end", req.End)
	} else {
		params.Set("end", fmt.Sprintf("%d", time.Now().Unix()))
	}
	if qType == "query_range" {
		params.Set("step", fmt.Sprintf("%d", step))
	}

	fullURL := upstreamURL + "?" + params.Encode()

	upReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fullURL, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "PROXY_ERROR", err.Error())
		return
	}
	for k, v := range ds.Headers {
		upReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(upReq)
	if err != nil {
		respondError(w, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		respondError(w, http.StatusBadGateway, "READ_ERROR", err.Error())
		return
	}

	var upstream interface{}
	if err := json.Unmarshal(body, &upstream); err != nil {
		respondError(w, http.StatusBadGateway, "PARSE_ERROR", "upstream returned non-JSON")
		return
	}

	respondJSON(w, http.StatusOK, models.APIResponse{
		Success:   true,
		Data:      upstream,
		Timestamp: time.Now(),
	})
}

// OrgListHandler lists all orgs.
func (h *Handlers) OrgListHandler(w http.ResponseWriter, r *http.Request) {
	var orgs []models.Org
	err := h.store.ListOrgs(func(val []byte) error {
		var o models.Org
		if err := json.Unmarshal(val, &o); err != nil {
			return err
		}
		orgs = append(orgs, o)
		return nil
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	if orgs == nil {
		orgs = []models.Org{}
	}
	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: orgs, Timestamp: time.Now()})
}

// OrgCreateHandler creates a new org.
func (h *Handlers) OrgCreateHandler(w http.ResponseWriter, r *http.Request) {
	var o models.Org
	if err := json.NewDecoder(r.Body).Decode(&o); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if o.Name == "" {
		respondError(w, http.StatusBadRequest, "MISSING_NAME", "name is required")
		return
	}
	o.ID = utils.GenerateID(8)
	if o.Slug == "" {
		o.Slug = slugify(o.Name)
	}
	now := time.Now()
	o.CreatedAt = now
	o.UpdatedAt = now

	if err := h.store.SaveOrg(o.ID, o); err != nil {
		respondError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, models.APIResponse{Success: true, Data: o, Timestamp: now})
}

// OrgGetHandler retrieves an org by ID.
func (h *Handlers) OrgGetHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var o models.Org
	if err := h.store.GetOrg(id, &o); err != nil {
		respondError(w, http.StatusNotFound, "ORG_NOT_FOUND", "org not found")
		return
	}
	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: o, Timestamp: time.Now()})
}

// OrgUpdateHandler updates an existing org.
func (h *Handlers) OrgUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var existing models.Org
	if err := h.store.GetOrg(id, &existing); err != nil {
		respondError(w, http.StatusNotFound, "ORG_NOT_FOUND", "org not found")
		return
	}
	var patch models.Org
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if patch.Name != "" {
		existing.Name = patch.Name
	}
	if patch.Slug != "" {
		existing.Slug = patch.Slug
	}
	if patch.Description != "" {
		existing.Description = patch.Description
	}
	existing.UpdatedAt = time.Now()

	if err := h.store.SaveOrg(id, existing); err != nil {
		respondError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Data: existing, Timestamp: existing.UpdatedAt})
}

// OrgDeleteHandler deletes an org.
func (h *Handlers) OrgDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.store.DeleteOrg(id); err != nil {
		respondError(w, http.StatusNotFound, "ORG_NOT_FOUND", "org not found")
		return
	}
	respondJSON(w, http.StatusOK, models.APIResponse{Success: true, Timestamp: time.Now()})
}

// slugify converts a name to a URL-safe slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
