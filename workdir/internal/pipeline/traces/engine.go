package traces

import (
	"fmt"
	"sync"

	"github.com/benfradjselim/kairo-core/internal/pipeline/metrics"
	"github.com/benfradjselim/kairo-core/pkg/models"
)

type Edge struct {
	From   string
	To     string
	Weight float64
}

type TracePipeline interface {
	IngestSpan(span models.Span) error
	CascadeIndex(host string) (float64, error)
	DependencyGraph(host string) ([]Edge, error)
}

type Engine struct {
	pipeline  metrics.MetricPipeline
	
	mu        sync.RWMutex
	edges     map[string]map[string]*edgeData
	services  map[string]*serviceData
	spanIDMap sync.Map
}

type edgeData struct {
	calls  int64
	errors int64
}

type serviceData struct {
	totalCalls int64
	errorCalls int64
}

func NewEngine(pipeline metrics.MetricPipeline) *Engine {
	return &Engine{
		pipeline: pipeline,
		edges:    make(map[string]map[string]*edgeData),
		services: make(map[string]*serviceData),
	}
}

func (e *Engine) IngestSpan(span models.Span) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.spanIDMap.Store(span.SpanID, span.Service)

	if _, ok := e.services[span.Service]; !ok {
		e.services[span.Service] = &serviceData{}
	}
	e.services[span.Service].totalCalls++
	if span.Status == "error" {
		e.services[span.Service].errorCalls++
	}

	if span.ParentID != "" {
		if parentService, ok := e.spanIDMap.Load(span.ParentID); ok {
			parentSvc := parentService.(string)
			if _, ok := e.edges[parentSvc]; !ok {
				e.edges[parentSvc] = make(map[string]*edgeData)
			}
			if _, ok := e.edges[parentSvc][span.Service]; !ok {
				e.edges[parentSvc][span.Service] = &edgeData{}
			}
			e.edges[parentSvc][span.Service].calls++
			if span.Status == "error" {
				e.edges[parentSvc][span.Service].errors++
			}
		}
	}
	return nil
}

func (e *Engine) CascadeIndex(host string) (float64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if _, ok := e.services[host]; !ok {
		return 0, fmt.Errorf("host not found")
	}
	return 0, nil
}

func (e *Engine) DependencyGraph(host string) ([]Edge, error) {
	return nil, nil
}
