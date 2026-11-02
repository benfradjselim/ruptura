package api

import (
	"net/http"

	"github.com/benfradjselim/ruptura/internal/normalize"
)

// InputCoverage reports which catalog roles were found.
type InputCoverage interface {
	Coverage() []normalize.Coverage
}

// SetInputs wires the input coverage into the API.
func (h *Handlers) SetInputs(inputs InputCoverage) { h.inputs = inputs }

func (h *Handlers) handleInputs(w http.ResponseWriter, _ *http.Request) {
	coverage := []normalize.Coverage{}
	if h.inputs != nil {
		coverage = h.inputs.Coverage()
	}
	writeJSON(w, http.StatusOK, coverage)
}
