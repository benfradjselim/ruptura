package context

import (
    "fmt"
    "sync"
    "time"
)

// ContextType represents a manual context label.
type ContextType string

const (
    ContextLoadTest         ContextType = "load_test"
    ContextMaintenanceWindow ContextType = "maintenance_window"
    ContextIncidentActive   ContextType = "incident_active"
    ContextAbnormalTraffic  ContextType = "abnormal_traffic"
)

// ContextEntry is a manually-set context record.
type ContextEntry struct {
    ID        string
    Type      ContextType
    Service   string
    Note      string
    CreatedAt time.Time
    ExpiresAt time.Time // TTL-based; zero means no expiry
}

// ManualContextStore stores manually-registered context entries with TTL.
type ManualContextStore struct {
    mu      sync.RWMutex
    entries map[string]ContextEntry
}

func NewManualContextStore() *ManualContextStore {
    return &ManualContextStore{entries: make(map[string]ContextEntry)}
}
func (s *ManualContextStore) Add(e ContextEntry) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if _, exists := s.entries[e.ID]; exists {
        return fmt.Errorf("id exists")
    }
    s.entries[e.ID] = e
    return nil
}
func (s *ManualContextStore) Get(id string) (ContextEntry, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    e, exists := s.entries[id]
    return e, exists
}
func (s *ManualContextStore) Delete(id string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if _, exists := s.entries[id]; !exists {
        return fmt.Errorf("not found")
    }
    delete(s.entries, id)
    return nil
}
func (s *ManualContextStore) List() []ContextEntry {
    s.mu.RLock()
    defer s.mu.RUnlock()
    now := time.Now()
    var res []ContextEntry
    for _, e := range s.entries {
        if e.ExpiresAt.IsZero() || e.ExpiresAt.After(now) {
            res = append(res, e)
        }
    }
    return res
}
func (s *ManualContextStore) Prune(now time.Time) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for id, e := range s.entries {
        if !e.ExpiresAt.IsZero() && e.ExpiresAt.Before(now) {
            delete(s.entries, id)
        }
    }
}
