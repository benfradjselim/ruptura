package api_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"net/http"
	"testing"

	"github.com/golang/snappy"
)

// encodeVarint writes v as a protobuf varint into buf and returns it.
func encodeVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

// encodeTag encodes field<<3|wireType.
func encodeTag(buf []byte, field, wireType uint64) []byte {
	return encodeVarint(buf, field<<3|wireType)
}

// buildWriteRequest constructs a minimal Prometheus WriteRequest protobuf:
//
//	TimeSeries { labels: [{name,value}...], samples: [{value, ts_ms}] }
func buildWriteRequest(metricName, host string, val float64, tsMS int64) []byte {
	// Build Sample: field1=double(val), field2=varint(tsMS)
	var sample []byte
	sample = encodeTag(sample, 1, 1) // field 1, wire type 1 (64-bit)
	bits := math.Float64bits(val)
	var fbuf [8]byte
	binary.LittleEndian.PutUint64(fbuf[:], bits)
	sample = append(sample, fbuf[:]...)
	sample = encodeTag(sample, 2, 0) // field 2, wire type 0 (varint)
	sample = encodeVarint(sample, uint64(tsMS))

	// Build Label {name: "__name__", value: metricName}
	nameLabel := buildLabel("__name__", metricName)
	// Build Label {name: "instance", value: host}
	instanceLabel := buildLabel("instance", host)

	// Build TimeSeries: field1=label, field1=label, field2=sample
	var ts []byte
	ts = appendLenField(ts, 1, nameLabel)
	ts = appendLenField(ts, 1, instanceLabel)
	ts = appendLenField(ts, 2, sample)

	// Build WriteRequest: field1=timeseries
	var wr []byte
	wr = appendLenField(wr, 1, ts)
	return wr
}

func buildLabel(name, value string) []byte {
	var b []byte
	b = appendLenField(b, 1, []byte(name))
	b = appendLenField(b, 2, []byte(value))
	return b
}

func appendLenField(buf []byte, fieldNum uint64, data []byte) []byte {
	buf = encodeTag(buf, fieldNum, 2) // wire type 2 = LEN
	buf = encodeVarint(buf, uint64(len(data)))
	return append(buf, data...)
}

// TestRemoteWritePlainProto sends a raw (uncompressed) protobuf WriteRequest.
func TestRemoteWritePlainProto(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	proto := buildWriteRequest("cpu_usage", "web-01", 72.5, 1700000000000)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/write", bytes.NewReader(proto))
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d; want 204", resp.StatusCode)
	}
}

// TestRemoteWriteSnappy sends a snappy-compressed WriteRequest (the real Prometheus format).
func TestRemoteWriteSnappy(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	proto := buildWriteRequest("mem_usage", "db-01", 55.0, 1700000001000)
	compressed := snappy.Encode(nil, proto)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/write", bytes.NewReader(compressed))
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST snappy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d; want 204", resp.StatusCode)
	}
	if resp.Header.Get("X-OHE-Samples-Written") == "" {
		t.Error("expected X-OHE-Samples-Written header")
	}
}

// TestRemoteWriteInvalidSnappy sends corrupt snappy data.
func TestRemoteWriteInvalidSnappy(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/write", bytes.NewReader([]byte("not snappy")))
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", resp.StatusCode)
	}
}

// TestRemoteWriteEmptyBody sends an empty body (zero time series).
func TestRemoteWriteEmptyBody(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/write", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/x-protobuf")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d; want 204", resp.StatusCode)
	}
}
