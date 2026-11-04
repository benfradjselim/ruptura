package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"
)

const testLeasePath = "/apis/coordination.k8s.io/v1/namespaces/ns/leases/ri-operator"

// leaseServer serves one Lease with resourceVersion checks, like the API server.
type leaseServer struct {
	mu      sync.Mutex
	lease   *Lease
	version int
	writes  int
}

func (s *leaseServer) handlers() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/apis/coordination.k8s.io/v1/namespaces/ns/leases": func(w http.ResponseWriter, r *http.Request) {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.lease != nil {
				w.WriteHeader(http.StatusConflict)
				return
			}
			s.store(w, r)
		},
		testLeasePath: func(w http.ResponseWriter, r *http.Request) {
			s.mu.Lock()
			defer s.mu.Unlock()
			switch r.Method {
			case http.MethodGet:
				if s.lease == nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				_ = json.NewEncoder(w).Encode(s.lease)
			case http.MethodPut:
				var l Lease
				_ = json.NewDecoder(r.Body).Decode(&l)
				if s.lease == nil || l.Metadata.ResourceVersion != s.lease.Metadata.ResourceVersion {
					w.WriteHeader(http.StatusConflict)
					return
				}
				s.save(l, w)
			case http.MethodDelete:
				s.lease = nil
			}
		},
	}
}

func (s *leaseServer) store(w http.ResponseWriter, r *http.Request) {
	var l Lease
	_ = json.NewDecoder(r.Body).Decode(&l)
	s.save(l, w)
}

func (s *leaseServer) save(l Lease, w http.ResponseWriter) {
	s.version++
	s.writes++
	l.Metadata.ResourceVersion = string(rune('a' + s.version))
	s.lease = &l
	_ = json.NewEncoder(w).Encode(l)
}

func operatorIn(t *testing.T, s *leaseServer, ns string) *k8sClient {
	c := fakeServer(t, s.handlers())
	c.ns = ns
	return c
}

func TestClaim_FirstOperatorTakesTheInstance(t *testing.T) {
	s := &leaseServer{}
	inst := makeInstance("ri", "ns")
	now := time.Now()
	a, b := operatorIn(t, s, "openshift-operators"), operatorIn(t, s, "ruptura-system")
	if holder, err := claim(a, inst, now); err != nil || holder != "" {
		t.Fatalf("first claim: holder %q err %v, want it held", holder, err)
	}
	if holder, err := claim(b, inst, now.Add(30*time.Second)); err != nil || holder != "openshift-operators" {
		t.Fatalf("second operator: holder %q err %v, want openshift-operators", holder, err)
	}
	if holder, err := claim(a, inst, now.Add(60*time.Second)); err != nil || holder != "" {
		t.Fatalf("renewal: holder %q err %v", holder, err)
	}
	if s.lease.Spec.HolderIdentity != "openshift-operators" {
		t.Errorf("lease holder = %q", s.lease.Spec.HolderIdentity)
	}
}

func TestClaim_TakesOverAnExpiredLease(t *testing.T) {
	s := &leaseServer{}
	inst := makeInstance("ri", "ns")
	now := time.Now()
	a, b := operatorIn(t, s, "openshift-operators"), operatorIn(t, s, "ruptura-system")
	if _, err := claim(a, inst, now); err != nil {
		t.Fatal(err)
	}
	if holder, _ := claim(b, inst, now.Add(leaseDuration-time.Second)); holder == "" {
		t.Fatal("took a lease that is still valid")
	}
	if holder, err := claim(b, inst, now.Add(leaseDuration+time.Second)); err != nil || holder != "" {
		t.Fatalf("after expiry: holder %q err %v, want it taken", holder, err)
	}
	if holder, _ := claim(a, inst, now.Add(leaseDuration+2*time.Second)); holder != "ruptura-system" {
		t.Errorf("old holder sees %q, want ruptura-system", holder)
	}
}

func TestClaim_RaceOnCreateHasOneWinner(t *testing.T) {
	s := &leaseServer{}
	inst := makeInstance("ri", "ns")
	now := time.Now()
	a, b := operatorIn(t, s, "openshift-operators"), operatorIn(t, s, "ruptura-system")
	var wg sync.WaitGroup
	held := make([]bool, 2)
	for i, c := range []*k8sClient{a, b} {
		wg.Add(1)
		go func(i int, c *k8sClient) {
			defer wg.Done()
			holder, err := claim(c, inst, now)
			held[i] = err == nil && holder == ""
		}(i, c)
	}
	wg.Wait()
	if held[0] == held[1] {
		t.Errorf("held = %v, want exactly one winner", held)
	}
}

func TestReconcile_SkipsAnInstanceHeldElsewhere(t *testing.T) {
	s := &leaseServer{}
	inst := makeInstance("ri", "ns")
	if _, err := claim(operatorIn(t, s, "openshift-operators"), inst, time.Now()); err != nil {
		t.Fatal(err)
	}
	var touched []string
	h := s.handlers()
	h["/"] = func(w http.ResponseWriter, r *http.Request) {
		touched = append(touched, r.Method+" "+r.URL.Path)
		_, _ = w.Write([]byte(`{}`))
	}
	c := fakeServer(t, h)
	c.ns = "ruptura-system"
	if err := reconcile(context.Background(), c, inst, false); err != nil {
		t.Fatal(err)
	}
	if len(touched) != 0 {
		t.Errorf("non-holder touched %v", touched)
	}
}

func TestCleanup_RemovesTheLease(t *testing.T) {
	s := &leaseServer{}
	inst := makeInstance("ri", "ns")
	c := operatorIn(t, s, "openshift-operators")
	if _, err := claim(c, inst, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := cleanup(c, inst, false); err != nil {
		t.Fatal(err)
	}
	if s.lease != nil {
		t.Error("lease left after cleanup")
	}
}
