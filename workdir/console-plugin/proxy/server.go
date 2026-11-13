package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// maxBodyBytes bounds what is read from the engine.
const maxBodyBytes = 16 << 20

// routes maps the read endpoints to how their answer is filtered. Anything
// else is refused.
type route int

const (
	byWorkload route = iota // a list of items with a workload key
	bySeries                // one workload, named in the query
	byGraph                 // nodes and edges
)

var routes = map[string]route{
	"/api/v2/failures":         byWorkload,
	"/api/v2/failures/closest": byWorkload,
	"/api/v2/failures/series":  bySeries,
	"/api/v2/callgraph":        byGraph,
}

type handler struct {
	backend string
	key     string
	checker AccessChecker
	client  *http.Client
}

func newHandler(backend, key string, checker AccessChecker, timeout time.Duration) http.Handler {
	return &handler{backend: strings.TrimRight(backend, "/"), key: key, checker: checker, client: &http.Client{Timeout: timeout}}
}

// bearerToken returns the token of an Authorization: Bearer header.
func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && strings.EqualFold(auth[:7], "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	kind, known := routes[r.URL.Path]
	if !known {
		http.Error(w, "not proxied", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "read only", http.StatusMethodNotAllowed)
		return
	}
	token := bearerToken(r)
	if token == "" {
		http.Error(w, "no user token", http.StatusUnauthorized)
		return
	}
	filter := newNamespaceFilter(r.Context(), token, h.checker)

	if kind == bySeries {
		ok, err := filter.allowed(namespaceOf(r.URL.Query().Get("workload")))
		if h.failed(w, err) {
			return
		}
		if !ok {
			http.Error(w, "workload not visible", http.StatusForbidden)
			return
		}
	}

	body, status, err := h.fetch(r.Context(), r.URL.RequestURI())
	if err != nil {
		log.Printf("engine %s: %v", r.URL.Path, err)
		http.Error(w, "engine unreachable", http.StatusBadGateway)
		return
	}
	if status != http.StatusOK {
		log.Printf("engine %s: HTTP %d", r.URL.Path, status)
		http.Error(w, fmt.Sprintf("engine answered %d", status), http.StatusBadGateway)
		return
	}

	var out []byte
	switch kind {
	case byWorkload:
		out, err = filterByWorkload(body, filter)
	case byGraph:
		out, err = filterGraph(body, filter)
	default:
		out = body
	}
	if h.failed(w, err) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(out)
}

// failed answers an access or decoding error and reports whether it did.
func (h *handler) failed(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errUnauthenticated) {
		http.Error(w, "user token refused", http.StatusUnauthorized)
		return true
	}
	log.Printf("access check: %v", err)
	http.Error(w, "access check failed", http.StatusBadGateway)
	return true
}

// fetch reads one engine endpoint with the API key.
func (h *handler) fetch(ctx context.Context, requestURI string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.backend+requestURI, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+h.key)
	req.Header.Set("Accept", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}
