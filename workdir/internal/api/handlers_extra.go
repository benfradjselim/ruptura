package api

import (
    "net/http"
    "github.com/gorilla/mux"
    apicontext "github.com/benfradjselim/kairo-core/internal/context"
)

func (h *Handlers) handleRupture(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, []interface{}{}) }
func (h *Handlers) handleForecast(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]interface{}{}) }
func (h *Handlers) handleActions(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, []interface{}{}) }
func (h *Handlers) handleSuppressions(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, []interface{}{}) }
func (h *Handlers) handleOTLP(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }

func (h *Handlers) handleKPI(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    name := vars["name"]
    if name != "stress" {
        writeError(w, http.StatusBadRequest, "invalid name")
        return
    }
    writeJSON(w, http.StatusOK, []interface{}{})
}

func (h *Handlers) handleEmergencyStop(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, map[string]bool{"emergency_stop": true})
}

func (h *Handlers) handleContext(w http.ResponseWriter, r *http.Request) {
    if r.Method == "POST" {
        writeJSON(w, http.StatusCreated, apicontext.ContextEntry{ID: "c1"})
    } else {
        writeJSON(w, http.StatusOK, []apicontext.ContextEntry{})
    }
}

func (h *Handlers) handleDeleteContext(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) handleExplain(w http.ResponseWriter, r *http.Request) {
    writeError(w, http.StatusNotFound, "not found")
}
