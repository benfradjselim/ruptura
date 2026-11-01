package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/benfradjselim/ruptura/internal/normalize"
)

type fixedInputs []normalize.Coverage

func (f fixedInputs) Coverage() []normalize.Coverage { return f }

func TestHandleInputs(t *testing.T) {
	cases := []struct {
		name   string
		inputs InputCoverage
		want   int
	}{
		{"not configured", nil, 0},
		{"reports missing roles", fixedInputs{{Role: "disk.limit"}, {Role: "memory.usage", Targets: 2, Available: true}}, 2},
	}
	for _, c := range cases {
		h := &Handlers{}
		if c.inputs != nil {
			h.SetInputs(c.inputs)
		}
		rec := httptest.NewRecorder()
		h.handleInputs(rec, httptest.NewRequest(http.MethodGet, "/api/v2/inputs", nil))
		var got []normalize.Coverage
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("%s: %v: %s", c.name, err, rec.Body.String())
		}
		if len(got) != c.want {
			t.Errorf("%s: roles = %d, want %d", c.name, len(got), c.want)
		}
	}
}
