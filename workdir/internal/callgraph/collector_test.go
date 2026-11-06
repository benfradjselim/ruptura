package callgraph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	deployments = `{"metadata":{},"items":[
 {"metadata":{"name":"api","namespace":"shop"},"spec":{"template":{"metadata":{"labels":{"app":"api"}},"spec":{"containers":[{"env":[{"name":"DB_URL","value":"http://shop-db:8080"},{"name":"FROM_SECRET"}]}]}}}},
 {"metadata":{"name":"db","namespace":"shop"},"spec":{"template":{"metadata":{"labels":{"app":"db"}},"spec":{"containers":[{}]}}}}]}`
	services = `{"metadata":{},"items":[
 {"metadata":{"name":"shop-db","namespace":"shop"},"spec":{"selector":{"app":"db"}}},
 {"metadata":{"name":"shop-api","namespace":"shop"},"spec":{"selector":{"app":"api"}}}]}`
	routes  = `{"metadata":{},"items":[{"metadata":{"name":"front","namespace":"shop"},"spec":{"host":"shop.example.com","to":{"name":"shop-api"}}}]}`
	empty   = `{"metadata":{},"items":[]}`
	pageOne = `{"metadata":{"continue":"next"},"items":[{"metadata":{"name":"a","namespace":"n"},"spec":{"template":{"metadata":{"labels":{}},"spec":{"containers":[]}}}}]}`
	pageTwo = `{"metadata":{},"items":[{"metadata":{"name":"b","namespace":"n"},"spec":{"template":{"metadata":{"labels":{}},"spec":{"containers":[]}}}}]}`
)

func serve(t *testing.T, responses map[string]string) *Collector {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		key := r.URL.Path
		if r.URL.Query().Get("continue") != "" {
			key += "#2"
		}
		body, ok := responses[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return &Collector{apiBase: server.URL, token: "token", client: server.Client()}
}

func TestRefreshBuildsGraph(t *testing.T) {
	c := serve(t, map[string]string{
		"/apis/apps/v1/deployments":          deployments,
		"/apis/apps/v1/statefulsets":         empty,
		"/apis/apps/v1/daemonsets":           empty,
		"/api/v1/services":                   services,
		"/apis/route.openshift.io/v1/routes": routes,
	})
	if err := c.refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	edges := c.Graph().Edges
	if len(edges) != 2 {
		t.Fatalf("got %+v, want api to db and the route to api", edges)
	}
	if edges[0].From != "shop/Deployment/api" || edges[0].To != "shop/Deployment/db" || edges[1].Source != SourceRoute {
		t.Errorf("got %+v", edges)
	}
}

func TestRefreshFailsWithoutWorkloads(t *testing.T) {
	c := serve(t, map[string]string{})
	if err := c.refresh(context.Background()); err == nil {
		t.Fatal("want an error when workloads cannot be listed")
	}
}

func TestListFollowsPages(t *testing.T) {
	c := serve(t, map[string]string{"/apis/apps/v1/deployments": pageOne, "/apis/apps/v1/deployments#2": pageTwo})
	count := 0
	err := list(context.Background(), c, "apis/apps/v1/deployments", func(json.RawMessage) error { count++; return nil })
	if err != nil || count != 2 {
		t.Errorf("got %d items, err %v", count, err)
	}
}
