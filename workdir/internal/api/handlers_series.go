package api

import (
	"net/http"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

// maxWorkloadKeyLength bounds the workload query, namespace/kind/name.
const maxWorkloadKeyLength = 512

// MemorySeriesSource returns the memory readings behind a workload's estimate.
type MemorySeriesSource interface {
	MemorySeries(workload string) []failuremodes.PodSeries
}

type seriesPoint struct {
	At    int64   `json:"t"`
	Usage float64 `json:"usage"`
}

type podSeries struct {
	Pod    string        `json:"pod"`
	Points []seriesPoint `json:"points"`
}

type failureSeriesResponse struct {
	Workload string      `json:"workload"`
	Pods     []podSeries `json:"pods"`
}

// seriesResponse converts readings to milliseconds since the epoch, the unit of
// JavaScript dates.
func seriesResponse(workload string, series []failuremodes.PodSeries) failureSeriesResponse {
	response := failureSeriesResponse{Workload: workload, Pods: make([]podSeries, 0, len(series))}
	for _, pod := range series {
		points := make([]seriesPoint, len(pod.Points))
		for i, p := range pod.Points {
			points[i] = seriesPoint{At: p.At.UnixMilli(), Usage: p.Usage}
		}
		response.Pods = append(response.Pods, podSeries{Pod: pod.Pod, Points: points})
	}
	return response
}

func (h *Handlers) handleFailureSeries(w http.ResponseWriter, r *http.Request) {
	workload := r.URL.Query().Get("workload")
	if workload == "" || len(workload) > maxWorkloadKeyLength {
		writeError(w, http.StatusBadRequest, "workload is required, as namespace/kind/name")
		return
	}
	var series []failuremodes.PodSeries
	if source, ok := h.failureModes.(MemorySeriesSource); ok {
		series = source.MemorySeries(workload)
	}
	writeJSON(w, http.StatusOK, seriesResponse(workload, series))
}
