package api

// Prometheus remote_write endpoint.
//
// Prometheus sends POST /api/v1/write with:
//   Content-Type: application/x-protobuf
//   Content-Encoding: snappy
//   X-Prometheus-Remote-Write-Version: 0.1.0
//
// The body is a snappy-compressed serialised prometheus.WriteRequest protobuf.
// We decode it with a hand-rolled varint+wire-type parser so we don't need a
// generated .pb.go file — only the already-indirect github.com/golang/snappy.
//
// WriteRequest proto layout (field numbers that matter):
//   message WriteRequest { repeated TimeSeries timeseries = 1; }
//   message TimeSeries   { repeated Label labels = 1; repeated Sample samples = 2; }
//   message Label        { string name = 1; string value = 2; }
//   message Sample       { double value = 1; int64 timestamp_ms = 2; }

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/golang/snappy"

	"github.com/benfradjselim/kairo-core/pkg/logger"
)

// RemoteWriteHandler accepts Prometheus remote_write payloads and converts them
// to OHE metric observations stored via the regular ingest path.
func (h *Handlers) RemoteWriteHandler(w http.ResponseWriter, r *http.Request) {
	raw, err := readBody(w, r, 32<<20) // 32 MiB limit
	if err != nil {
		respondError(w, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "request body exceeds limit")
		return
	}

	// Decode snappy if the client sent it (Prometheus always does)
	var payload []byte
	if r.Header.Get("Content-Encoding") == "snappy" {
		payload, err = snappy.Decode(nil, raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "SNAPPY_DECODE", "snappy decode failed: "+err.Error())
			return
		}
	} else {
		payload = raw
	}

	series, err := parseWriteRequest(payload)
	if err != nil {
		respondError(w, http.StatusBadRequest, "PROTO_DECODE", "protobuf decode failed: "+err.Error())
		return
	}

	orgID := orgIDFromContext(r.Context())
	os := h.store.ForOrg(orgID)
	saved := 0
	for _, ts := range series {
		host := labelValue(ts.labels, "instance")
		if host == "" {
			host = labelValue(ts.labels, "job")
		}
		name := sanitizeMetricName(labelValue(ts.labels, "__name__"))
		if name == "" {
			continue
		}
		for _, s := range ts.samples {
			t := time.UnixMilli(s.timestampMS).UTC()
			if err := os.SaveMetric(host, name, s.value, t); err != nil {
				logger.Default.ErrorCtx(r.Context(), "remote_write save", "err", err)
			} else {
				saved++
			}
		}
	}

	h.recordUsage(r, "ingest_bytes", float64(len(raw)))
	w.Header().Set("X-OHE-Samples-Written", fmt.Sprintf("%d", saved))
	w.WriteHeader(http.StatusNoContent)
}

// --- minimal protobuf decoder ---

type protoSample struct {
	value       float64
	timestampMS int64
}

type protoTimeSeries struct {
	labels  map[string]string
	samples []protoSample
}

func labelValue(labels map[string]string, key string) string {
	return labels[key]
}

// parseWriteRequest decodes a prometheus WriteRequest protobuf.
// Only fields needed for ingestion are parsed; unknown fields are skipped.
func parseWriteRequest(b []byte) ([]protoTimeSeries, error) {
	var series []protoTimeSeries
	for len(b) > 0 {
		fieldNum, wireType, n, err := consumeTag(b)
		if err != nil {
			return nil, err
		}
		b = b[n:]
		switch {
		case fieldNum == 1 && wireType == 2: // TimeSeries (LEN)
			msg, rest, err := consumeBytes(b)
			if err != nil {
				return nil, err
			}
			b = rest
			ts, err := parseTimeSeries(msg)
			if err != nil {
				return nil, err
			}
			series = append(series, ts)
		default:
			b, err = skipField(b, wireType)
			if err != nil {
				return nil, err
			}
		}
	}
	return series, nil
}

func parseTimeSeries(b []byte) (protoTimeSeries, error) {
	ts := protoTimeSeries{labels: make(map[string]string)}
	for len(b) > 0 {
		fieldNum, wireType, n, err := consumeTag(b)
		if err != nil {
			return ts, err
		}
		b = b[n:]
		switch {
		case fieldNum == 1 && wireType == 2: // Label
			msg, rest, err := consumeBytes(b)
			if err != nil {
				return ts, err
			}
			b = rest
			k, v, err := parseLabel(msg)
			if err != nil {
				return ts, err
			}
			ts.labels[k] = v
		case fieldNum == 2 && wireType == 2: // Sample
			msg, rest, err := consumeBytes(b)
			if err != nil {
				return ts, err
			}
			b = rest
			s, err := parseSample(msg)
			if err != nil {
				return ts, err
			}
			ts.samples = append(ts.samples, s)
		default:
			b, err = skipField(b, wireType)
			if err != nil {
				return ts, err
			}
		}
	}
	return ts, nil
}

func parseLabel(b []byte) (string, string, error) {
	var name, value string
	for len(b) > 0 {
		fieldNum, wireType, n, err := consumeTag(b)
		if err != nil {
			return "", "", err
		}
		b = b[n:]
		if wireType != 2 {
			b, err = skipField(b, wireType)
			if err != nil {
				return "", "", err
			}
			continue
		}
		s, rest, err := consumeBytes(b)
		if err != nil {
			return "", "", err
		}
		b = rest
		switch fieldNum {
		case 1:
			name = string(s)
		case 2:
			value = string(s)
		}
	}
	return name, value, nil
}

func parseSample(b []byte) (protoSample, error) {
	var s protoSample
	for len(b) > 0 {
		fieldNum, wireType, n, err := consumeTag(b)
		if err != nil {
			return s, err
		}
		b = b[n:]
		switch {
		case fieldNum == 1 && wireType == 1: // double (fixed64)
			if len(b) < 8 {
				return s, io.ErrUnexpectedEOF
			}
			bits := binary.LittleEndian.Uint64(b[:8])
			s.value = math.Float64frombits(bits)
			b = b[8:]
		case fieldNum == 2 && wireType == 0: // int64 (varint)
			v, n, err := consumeVarint(b)
			if err != nil {
				return s, err
			}
			s.timestampMS = int64(v)
			b = b[n:]
		default:
			b, err = skipField(b, wireType)
			if err != nil {
				return s, err
			}
		}
	}
	return s, nil
}

// consumeTag reads a protobuf tag varint and returns (fieldNumber, wireType, bytesRead, error).
func consumeTag(b []byte) (uint64, uint64, int, error) {
	v, n, err := consumeVarint(b)
	if err != nil {
		return 0, 0, 0, err
	}
	return v >> 3, v & 0x7, n, nil
}

// consumeVarint reads a protobuf varint, returning (value, bytesRead, error).
func consumeVarint(b []byte) (uint64, int, error) {
	var x uint64
	for i, bt := range b {
		if i >= 10 {
			return 0, 0, fmt.Errorf("varint overflow")
		}
		x |= uint64(bt&0x7f) << (7 * uint(i))
		if bt < 0x80 {
			return x, i + 1, nil
		}
	}
	return 0, 0, io.ErrUnexpectedEOF
}

// consumeBytes reads a length-delimited field, returning (data, rest, error).
func consumeBytes(b []byte) ([]byte, []byte, error) {
	length, n, err := consumeVarint(b)
	if err != nil {
		return nil, nil, err
	}
	b = b[n:]
	if uint64(len(b)) < length {
		return nil, nil, io.ErrUnexpectedEOF
	}
	return b[:length], b[length:], nil
}

// skipField advances past a field of the given wire type.
func skipField(b []byte, wireType uint64) ([]byte, error) {
	switch wireType {
	case 0: // varint
		_, n, err := consumeVarint(b)
		if err != nil {
			return nil, err
		}
		return b[n:], nil
	case 1: // 64-bit
		if len(b) < 8 {
			return nil, io.ErrUnexpectedEOF
		}
		return b[8:], nil
	case 2: // length-delimited
		_, rest, err := consumeBytes(b)
		return rest, err
	case 5: // 32-bit
		if len(b) < 4 {
			return nil, io.ErrUnexpectedEOF
		}
		return b[4:], nil
	default:
		return nil, fmt.Errorf("unknown wire type %d", wireType)
	}
}
