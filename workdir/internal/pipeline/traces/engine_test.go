package traces

import (
	"testing"

	"github.com/benfradjselim/kairo-core/pkg/models"
)

func TestCascadeIndex_noData(t *testing.T) {
	e := NewEngine(nil)
	_, err := e.CascadeIndex("svc1")
	if err == nil {
		t.Error("expected error for empty engine")
	}
}

func TestTopologyBuilder_basic(t *testing.T) {
	e := NewEngine(nil)
	e.IngestSpan(models.Span{Service: "s1", SpanID: "1"})
	e.IngestSpan(models.Span{Service: "s2", SpanID: "2", ParentID: "1"})
}
