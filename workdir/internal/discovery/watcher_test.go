package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// listFixture builds a LIST response body with the given names in the given namespace.
func listFixture(ns string, names []string, rv string) []byte {
	items := make([]k8sObjectMeta, len(names))
	for i, name := range names {
		items[i].Metadata.Name = name
		items[i].Metadata.Namespace = ns
		items[i].Metadata.ResourceVersion = fmt.Sprintf("100%d", i)
	}
	lr := listResponse{}
	lr.Metadata.ResourceVersion = rv
	lr.Items = items
	b, _ := json.Marshal(lr)
	return b
}

// streamEvent encodes a single watch event as a newline-terminated JSON line.
func streamEvent(evType, ns, name, rv string) []byte {
	ev := watchEvent{Type: evType}
	ev.Object.Metadata.Namespace = ns
	ev.Object.Metadata.Name = name
	ev.Object.Metadata.ResourceVersion = rv
	b, _ := json.Marshal(ev)
	return append(b, '\n')
}

// TestWatchResource_ListCallsOnAdd verifies that a LIST response triggers onAdd
// for every item.
func TestWatchResource_ListCallsOnAdd(t *testing.T) {
	var mu sync.Mutex
	var added []models.WorkloadRef

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") == "true" {
			// Keep the watch connection open until the test context is done.
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(listFixture("prod", []string{"api", "worker"}, "42"))
	}))
	defer srv.Close()

	inf := newInformerForTest(srv.URL, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res := resource{path: "apis/apps/v1/deployments", kind: "Deployment"}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		inf.watchResource(ctx, res,
			func(ref models.WorkloadRef) {
				mu.Lock()
				added = append(added, ref)
				mu.Unlock()
			},
			func(models.WorkloadRef) {},
			nil,
		)
	}()

	// Wait until at least 2 onAdd calls (from the list).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(added)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(added) < 2 {
		t.Fatalf("want >= 2 onAdd calls, got %d", len(added))
	}
	names := map[string]bool{}
	for _, r := range added {
		names[r.Name] = true
	}
	if !names["api"] || !names["worker"] {
		t.Errorf("missing expected workload names in %v", added)
	}
}

// TestWatchResource_WatchEventsRoutedCorrectly verifies ADDED→onAdd, DELETED→onDelete.
func TestWatchResource_WatchEventsRoutedCorrectly(t *testing.T) {
	var mu sync.Mutex
	var added, deleted []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") != "true" {
			w.Write(listFixture("ns", nil, "1"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter does not implement Flusher")
			return
		}
		w.Write(streamEvent("ADDED", "ns", "svc-a", "2"))
		flusher.Flush()
		w.Write(streamEvent("DELETED", "ns", "svc-b", "3"))
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer srv.Close()

	inf := newInformerForTest(srv.URL, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res := resource{path: "apis/apps/v1/deployments", kind: "Deployment"}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		inf.watchResource(ctx, res,
			func(ref models.WorkloadRef) { mu.Lock(); added = append(added, ref.Name); mu.Unlock() },
			func(ref models.WorkloadRef) { mu.Lock(); deleted = append(deleted, ref.Name); mu.Unlock() },
			nil,
		)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		a, d := len(added), len(deleted)
		mu.Unlock()
		if a >= 1 && d >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(added) == 0 || added[0] != "svc-a" {
		t.Errorf("expected added[0]=svc-a, got %v", added)
	}
	if len(deleted) == 0 || deleted[0] != "svc-b" {
		t.Errorf("expected deleted[0]=svc-b, got %v", deleted)
	}
}

// TestWatchResource_410Gone_TriggersRelist verifies that a 410 response causes a
// fresh LIST to be issued (we see a second round of onAdd calls).
func TestWatchResource_410Gone_TriggersRelist(t *testing.T) {
	var mu sync.Mutex
	listCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") != "true" {
			mu.Lock()
			listCount++
			mu.Unlock()
			w.Write(listFixture("ns", []string{"svc"}, "5"))
			return
		}
		// Return 410 Gone — simulates resourceVersion too old.
		http.Error(w, `{"kind":"Status","code":410}`, http.StatusGone)
	}))
	defer srv.Close()

	inf := newInformerForTest(srv.URL, srv.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res := resource{path: "apis/apps/v1/deployments", kind: "Deployment"}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		inf.watchResource(ctx, res, func(models.WorkloadRef) {}, func(models.WorkloadRef) {}, nil)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := listCount
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if listCount < 2 {
		t.Errorf("expected at least 2 LIST calls after 410, got %d", listCount)
	}
}

// TestWatchResource_ContextCancel ensures the goroutine exits cleanly on cancel.
func TestWatchResource_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") == "true" {
			<-r.Context().Done()
			return
		}
		w.Write(listFixture("ns", nil, "1"))
	}))
	defer srv.Close()

	inf := newInformerForTest(srv.URL, srv.Client())
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		res := resource{path: "apis/apps/v1/deployments", kind: "Deployment"}
		inf.watchResource(ctx, res, func(models.WorkloadRef) {}, func(models.WorkloadRef) {}, nil)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchResource did not exit after context cancel")
	}
}
