package telemetry

import (
    "fmt"
    "math"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

type Registry struct {
    startTime time.Time
    version   string

    ruptureIndex     sync.Map // key="host:metric:severity" -> uint64 (bits)
    timeToFailure    sync.Map // key="host:metric" -> uint64 (bits)
    predictedValue   sync.Map // key="host:metric:horizon" -> uint64 (bits)
    confidence       sync.Map // key="host" -> uint64 (bits)
    fusedProbability sync.Map // key="host" -> uint64 (bits)
    kpiStress        sync.Map // key="host" -> uint64 (bits)
    kpiFatigue       sync.Map // key="host" -> uint64 (bits)
    kpiHealthscore   sync.Map // key="host" -> uint64 (bits)
    trackerCount     sync.Map // key="type:state" -> *int64

    actionsTotal sync.Map // key="type:tier:outcome" -> *int64
    ingestTotal  sync.Map // key="source" -> *int64

    memoryBytes int64 // atomic
}

func NewRegistry(version string) *Registry {
    return &Registry{startTime: time.Now(), version: version}
}

func (r *Registry) setFloat(m *sync.Map, key string, v float64) {
    m.Store(key, math.Float64bits(v))
}

func (r *Registry) SetRuptureIndex(host, metric, severity string, v float64) {
    r.setFloat(&r.ruptureIndex, host+":"+metric+":"+severity, v)
}
func (r *Registry) SetTimeToFailure(host, metric string, v float64) {
    r.setFloat(&r.timeToFailure, host+":"+metric, v)
}
func (r *Registry) SetPredictedValue(host, metric, horizon string, v float64) {
    r.setFloat(&r.predictedValue, host+":"+metric+":"+horizon, v)
}
func (r *Registry) SetConfidence(host string, v float64) {
    r.setFloat(&r.confidence, host, v)
}
func (r *Registry) SetFusedProbability(host string, v float64) {
    r.setFloat(&r.fusedProbability, host, v)
}
func (r *Registry) SetKPIStress(host string, v float64) {
    r.setFloat(&r.kpiStress, host, v)
}
func (r *Registry) SetKPIFatigue(host string, v float64) {
    r.setFloat(&r.kpiFatigue, host, v)
}
func (r *Registry) SetKPIHealthscore(host string, v float64) {
    r.setFloat(&r.kpiHealthscore, host, v)
}
func (r *Registry) SetTrackerCount(trackerType, state string, v int64) {
    key := trackerType + ":" + state
    actual, _ := r.trackerCount.LoadOrStore(key, new(int64))
    atomic.StoreInt64(actual.(*int64), v)
}
func (r *Registry) SetMemoryBytes(v int64) { atomic.StoreInt64(&r.memoryBytes, v) }

func (r *Registry) IncActionsTotal(actionType, tier, outcome string) {
    key := actionType + ":" + tier + ":" + outcome
    actual, _ := r.actionsTotal.LoadOrStore(key, new(int64))
    atomic.AddInt64(actual.(*int64), 1)
}
func (r *Registry) IncIngestTotal(source string) {
    actual, _ := r.ingestTotal.LoadOrStore(source, new(int64))
    atomic.AddInt64(actual.(*int64), 1)
}

func (r *Registry) Render() string {
    var b strings.Builder
    
    // Helper to render gauges
    renderGauge := func(name, help string, m *sync.Map, keys ...string) {
        b.WriteString(fmt.Sprintf("# HELP %s %s\n# TYPE %s gauge\n", name, help, name))
        m.Range(func(key, val interface{}) bool {
            parts := strings.Split(key.(string), ":")
            labels := ""
            for i, p := range parts {
                labels += fmt.Sprintf(`%s="%s",`, keys[i], p)
            }
            labels = labels[:len(labels)-1]
            v := math.Float64frombits(val.(uint64))
            b.WriteString(fmt.Sprintf("%s{%s} %f\n", name, labels, v))
            return true
        })
    }
    
    renderGauge("rpt_rupture_index", "Current rupture index per host/metric", &r.ruptureIndex, "host", "metric", "severity")
    renderGauge("rpt_time_to_failure_seconds", "Time to failure", &r.timeToFailure, "host", "metric")
    renderGauge("rpt_predicted_value", "Predicted value", &r.predictedValue, "host", "metric", "horizon")
    renderGauge("rpt_confidence", "Confidence", &r.confidence, "host")
    renderGauge("rpt_fused_rupture_probability", "Fused rupture probability", &r.fusedProbability, "host")
    renderGauge("rpt_kpi_stress", "KPI stress", &r.kpiStress, "host")
    renderGauge("rpt_kpi_fatigue", "KPI fatigue", &r.kpiFatigue, "host")
    renderGauge("rpt_kpi_healthscore", "KPI healthscore", &r.kpiHealthscore, "host")
    
    // Counters (rudimentary)
    b.WriteString("# HELP rpt_actions_total Actions total\n# TYPE rpt_actions_total counter\n")
    r.actionsTotal.Range(func(key, val interface{}) bool {
        b.WriteString(fmt.Sprintf("rpt_actions_total{%s} %d\n", key.(string), atomic.LoadInt64(val.(*int64))))
        return true
    })
    
    b.WriteString("# HELP rpt_ingest_samples_total Ingest total\n# TYPE rpt_ingest_samples_total counter\n")
    r.ingestTotal.Range(func(key, val interface{}) bool {
        b.WriteString(fmt.Sprintf("rpt_ingest_samples_total{source=\"%s\"} %d\n", key.(string), atomic.LoadInt64(val.(*int64))))
        return true
    })
    
    b.WriteString(fmt.Sprintf("rpt_memory_bytes %d\n", atomic.LoadInt64(&r.memoryBytes)))
    b.WriteString(fmt.Sprintf("rpt_uptime_seconds %f\n", time.Since(r.startTime).Seconds()))
    b.WriteString(fmt.Sprintf("rpt_version_info{version=\"%s\"} 1\n", r.version))
    
    return b.String()
}
