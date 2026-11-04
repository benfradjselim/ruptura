package ingest

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/golang/snappy"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/benfradjselim/ruptura/pkg/models"
)

const maxRemoteWriteBody = 32 << 20

type promLabel struct{ Name, Value string }

type promSample struct {
	Value     float64
	Timestamp int64
}

type promSeries struct {
	Labels  []promLabel
	Samples []promSample
}

// handlePrometheusRemoteWrite accepts Prometheus remote write 1.0: a
// snappy-compressed protobuf WriteRequest.
func (e *Engine) handlePrometheusRemoteWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	compressed, err := io.ReadAll(io.LimitReader(r.Body, maxRemoteWriteBody+1))
	if err != nil || len(compressed) > maxRemoteWriteBody {
		http.Error(w, "body too large or unreadable", http.StatusBadRequest)
		return
	}
	raw, err := snappy.Decode(nil, compressed)
	if err != nil {
		http.Error(w, "body is not snappy compressed", http.StatusBadRequest)
		return
	}
	series, err := decodeWriteRequest(raw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, ts := range series {
		e.ingestSeries(ts.Labels, ts.Samples)
	}
	w.WriteHeader(http.StatusNoContent)
}

// ingestSeries maps one labelled series to a host and feeds its samples.
func (e *Engine) ingestSeries(labels []promLabel, samples []promSample) {
	var name string
	host := "unknown"
	workload := models.WorkloadRef{}
	for _, lbl := range labels {
		switch lbl.Name {
		case "__name__":
			name = lbl.Value
		case "host", "instance":
			host = lbl.Value
		case "namespace":
			workload.Namespace = lbl.Value
		case "deployment":
			workload.Name = lbl.Value
			workload.Kind = "Deployment"
		}
	}
	if name == "" {
		return
	}
	if workload.IsEmpty() {
		workload = models.WorkloadRefFromHost(host)
	}
	if !e.checkCardinality(host, name) {
		return
	}
	for _, s := range samples {
		if math.IsNaN(s.Value) {
			continue
		}
		e.pipeline.Ingest(host, name, s.Value, time.UnixMilli(s.Timestamp))
		atomic.AddInt64(&e.metricsCount, 1)
	}
}

var errWire = errors.New("remote write: malformed protobuf")

// fields walks the fields of one protobuf message.
func fields(b []byte, visit func(num protowire.Number, typ protowire.Type, v []byte, n uint64) error) error {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return errWire
		}
		b = b[n:]
		var value []byte
		var scalar uint64
		switch typ {
		case protowire.BytesType:
			value, n = protowire.ConsumeBytes(b)
		case protowire.VarintType:
			scalar, n = protowire.ConsumeVarint(b)
		case protowire.Fixed64Type:
			scalar, n = protowire.ConsumeFixed64(b)
		case protowire.Fixed32Type:
			var v32 uint32
			v32, n = protowire.ConsumeFixed32(b)
			scalar = uint64(v32)
		default:
			n = protowire.ConsumeFieldValue(num, typ, b)
		}
		if n < 0 {
			return errWire
		}
		if err := visit(num, typ, value, scalar); err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

// decodeWriteRequest reads the series of a prometheus.WriteRequest. Metadata,
// exemplars and native histograms are skipped.
func decodeWriteRequest(b []byte) ([]promSeries, error) {
	var out []promSeries
	err := fields(b, func(num protowire.Number, typ protowire.Type, v []byte, _ uint64) error {
		if num != 1 || typ != protowire.BytesType {
			return nil
		}
		var ts promSeries
		err := fields(v, func(num protowire.Number, typ protowire.Type, v []byte, _ uint64) error {
			if typ != protowire.BytesType {
				return nil
			}
			switch num {
			case 1:
				var l promLabel
				err := fields(v, func(num protowire.Number, typ protowire.Type, v []byte, _ uint64) error {
					if typ == protowire.BytesType && num == 1 {
						l.Name = string(v)
					} else if typ == protowire.BytesType && num == 2 {
						l.Value = string(v)
					}
					return nil
				})
				ts.Labels = append(ts.Labels, l)
				return err
			case 2:
				var s promSample
				err := fields(v, func(num protowire.Number, typ protowire.Type, _ []byte, scalar uint64) error {
					if num == 1 && typ == protowire.Fixed64Type {
						s.Value = math.Float64frombits(scalar)
					} else if num == 2 && typ == protowire.VarintType {
						s.Timestamp = int64(scalar)
					}
					return nil
				})
				ts.Samples = append(ts.Samples, s)
				return err
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("remote write: series %d: %w", len(out), err)
		}
		out = append(out, ts)
		return nil
	})
	return out, err
}
