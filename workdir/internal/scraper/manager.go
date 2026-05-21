package scraper

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	pipelinemetrics "github.com/benfradjselim/ruptura/internal/pipeline/metrics"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

// Storer is the subset of storage.Store needed by Manager.
type Storer interface {
	PutDatasource(id string, data []byte) error
	DeleteDatasource(id string) error
	ListDatasources() ([][]byte, error)
}

// Manager runs per-datasource scrape goroutines and persists configs.
type Manager struct {
	mu       sync.RWMutex
	pipeline pipelinemetrics.MetricPipeline
	store    Storer
	client   *http.Client

	ds     map[string]*dsState // id → state
	stopCh chan struct{}
}

type dsState struct {
	cfg        DatasourceConfig
	status     string
	lastScrape time.Time
	lastErr    string
	scraped    int

	cancel chan struct{}
}

// New creates a Manager. Call Start() to begin scraping.
func New(pipeline pipelinemetrics.MetricPipeline, store Storer) *Manager {
	return &Manager{
		pipeline: pipeline,
		store:    store,
		client:   &http.Client{Timeout: 15 * time.Second},
		ds:       make(map[string]*dsState),
		stopCh:   make(chan struct{}),
	}
}

// Start loads persisted configs and begins scrape loops.
func (m *Manager) Start() {
	if m.store == nil {
		return
	}
	rows, err := m.store.ListDatasources()
	if err != nil {
		logger.Default.Error("scraper: failed to load datasources", "error", err)
		return
	}
	for _, row := range rows {
		var cfg DatasourceConfig
		if err := json.Unmarshal(row, &cfg); err != nil {
			continue
		}
		m.startDS(&cfg)
	}
}

// Stop shuts down all scrape loops.
func (m *Manager) Stop() {
	close(m.stopCh)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.ds {
		close(s.cancel)
	}
}

// List returns the current status of all datasources.
func (m *Manager) List() []DatasourceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]DatasourceStatus, 0, len(m.ds))
	for _, s := range m.ds {
		out = append(out, m.statusOf(s))
	}
	return out
}

// Get returns the status of a single datasource.
func (m *Manager) Get(id string) (DatasourceStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.ds[id]
	if !ok {
		return DatasourceStatus{}, false
	}
	return m.statusOf(s), true
}

func (m *Manager) statusOf(s *dsState) DatasourceStatus {
	st := s.status
	if st == "" {
		st = "pending"
	}
	if !s.cfg.Enabled {
		st = "disabled"
	}
	return DatasourceStatus{
		DatasourceConfig: s.cfg,
		Status:           st,
		LastScrape:       s.lastScrape,
		LastError:        s.lastErr,
		ScrapedMetrics:   s.scraped,
	}
}

// Put creates or replaces a datasource config and (re)starts its scrape loop.
func (m *Manager) Put(cfg DatasourceConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("datasource ID is required")
	}
	cfg.UpdatedAt = time.Now()
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = cfg.UpdatedAt
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if m.store != nil {
		if err := m.store.PutDatasource(cfg.ID, data); err != nil {
			return err
		}
	}

	m.mu.Lock()
	existing, exists := m.ds[cfg.ID]
	if exists {
		close(existing.cancel) // stop old loop
	}
	m.mu.Unlock()

	m.startDS(&cfg)
	return nil
}

// Delete removes a datasource and stops its scrape loop.
func (m *Manager) Delete(id string) error {
	if m.store != nil {
		if err := m.store.DeleteDatasource(id); err != nil {
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.ds[id]
	if ok {
		close(s.cancel)
		delete(m.ds, id)
	}
	return nil
}

// Test performs a single scrape of the given config without persisting or running a loop.
// Returns (count of mapped metrics, error string).
func (m *Manager) Test(cfg DatasourceConfig) (int, string) {
	if cfg.Type == TypeOTLP {
		return testOTLPConnectivity(cfg.URL)
	}
	samples, err := m.runScrape(&cfg)
	if err != nil {
		return 0, err.Error()
	}
	return len(samples), ""
}

// testOTLPConnectivity checks that the OTLP endpoint (Ruptura's own NodePort) is reachable via TCP.
func testOTLPConnectivity(rawURL string) (int, string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, "invalid URL: " + err.Error()
	}
	host := u.Host
	if host == "" {
		return 0, "URL must include host:port"
	}
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		return 0, fmt.Sprintf("OTLP endpoint unreachable: %v", err)
	}
	conn.Close()
	return 0, ""
}

// startDS registers state and launches the scrape goroutine.
func (m *Manager) startDS(cfg *DatasourceConfig) {
	st := "pending"
	if cfg.Type == TypeOTLP {
		st = "push-only"
	}
	state := &dsState{
		cfg:    *cfg,
		status: st,
		cancel: make(chan struct{}),
	}
	m.mu.Lock()
	m.ds[cfg.ID] = state
	m.mu.Unlock()

	if !cfg.Enabled {
		return
	}
	if cfg.Type == TypeOTLP {
		return // push-based: clients push to Ruptura's OTLP port; no poll loop needed
	}
	go m.scrapeLoop(state)
}

func (m *Manager) scrapeLoop(state *dsState) {
	ticker := time.NewTicker(state.cfg.scrapeInterval())
	defer ticker.Stop()

	// scrape immediately on start
	m.doScrape(state)

	for {
		select {
		case <-ticker.C:
			m.doScrape(state)
		case <-state.cancel:
			return
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) doScrape(state *dsState) {
	samples, err := m.runScrape(&state.cfg)

	m.mu.Lock()
	state.lastScrape = time.Now()
	if err != nil {
		state.status = "error"
		state.lastErr = err.Error()
		state.scraped = 0
	} else {
		state.status = "ok"
		state.lastErr = ""
		state.scraped = len(samples)
	}
	m.mu.Unlock()

	if err != nil {
		logger.Default.Error("scraper: scrape failed",
			"id", state.cfg.ID,
			"type", state.cfg.Type,
			"url", state.cfg.URL,
			"error", err)
		return
	}

	now := time.Now()
	for _, s := range samples {
		m.pipeline.Ingest(s.workload, s.metric, s.value, now)
	}
}

func (m *Manager) runScrape(cfg *DatasourceConfig) ([]promSample, error) {
	switch cfg.Type {
	case TypePrometheus:
		return scrapePrometheus(cfg, m.client)
	case TypeDirect:
		return scrapeDirect(cfg, m.client)
	case TypeOTLP:
		// OTLP is push-only — nothing to scrape actively
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown datasource type: %s", cfg.Type)
	}
}
