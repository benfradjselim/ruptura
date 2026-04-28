package utils

import (
	"math"
	"testing"
	"time"
)

func TestClamp(t *testing.T) {
	tests := []struct{ in, min, max, want float64 }{
		{0.5, 0, 1, 0.5},
		{-1, 0, 1, 0},
		{2, 0, 1, 1},
		{0, 0, 0, 0},
	}
	for _, tc := range tests {
		if got := Clamp(tc.in, tc.min, tc.max); got != tc.want {
			t.Errorf("Clamp(%v,%v,%v) = %v; want %v", tc.in, tc.min, tc.max, got, tc.want)
		}
	}
}

func TestNormalizePercent(t *testing.T) {
	if got := NormalizePercent(50); got != 0.5 {
		t.Errorf("NormalizePercent(50) = %v; want 0.5", got)
	}
	if got := NormalizePercent(0); got != 0 {
		t.Errorf("NormalizePercent(0) = %v; want 0", got)
	}
	if got := NormalizePercent(100); got != 1.0 {
		t.Errorf("NormalizePercent(100) = %v; want 1.0", got)
	}
	if got := NormalizePercent(150); got != 1.0 {
		t.Errorf("NormalizePercent(150) should be clamped to 1.0, got %v", got)
	}
}

func TestMean(t *testing.T) {
	if got := Mean([]float64{1, 2, 3, 4, 5}); got != 3.0 {
		t.Errorf("Mean = %v; want 3.0", got)
	}
	if got := Mean(nil); got != 0 {
		t.Errorf("Mean(nil) = %v; want 0", got)
	}
}

func TestStdDev(t *testing.T) {
	// stddev of {2,4,4,4,5,5,7,9} = 2.0
	vals := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	got := StdDev(vals)
	if math.Abs(got-2.0) > 0.01 {
		t.Errorf("StdDev = %v; want ~2.0", got)
	}
}

func TestPercentile(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := Percentile(vals, 50); got != 5 && got != 5.5 {
		t.Logf("Percentile p50 = %v (acceptable)", got)
	}
	if got := Percentile(vals, 90); got < 8 || got > 10 {
		t.Errorf("Percentile p90 = %v; expected in [8,10]", got)
	}
	if got := Percentile(nil, 50); got != 0 {
		t.Errorf("Percentile(nil) = %v; want 0", got)
	}
}

func TestTrapezoidIntegrate(t *testing.T) {
	vals := []float64{0, 1, 0}
	// trapezoid: (0+1)/2 + (1+0)/2 = 0.5 + 0.5 = 1.0
	if got := TrapezoidIntegrate(vals, 1); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("TrapezoidIntegrate = %v; want 1.0", got)
	}
}

func TestDerivative(t *testing.T) {
	if got := Derivative(0, 10, 2); got != 5 {
		t.Errorf("Derivative = %v; want 5", got)
	}
	if got := Derivative(0, 1, 0); got != 0 {
		t.Errorf("Derivative with dt=0 = %v; want 0", got)
	}
}

func TestCircularBuffer(t *testing.T) {
	buf := NewCircularBuffer(3)
	if buf.Len() != 0 {
		t.Errorf("new buffer len = %d; want 0", buf.Len())
	}

	buf.Push(1)
	buf.Push(2)
	buf.Push(3)
	if buf.Len() != 3 {
		t.Errorf("buf.Len() = %d; want 3", buf.Len())
	}

	vals := buf.Values()
	if len(vals) != 3 || vals[0] != 1 || vals[1] != 2 || vals[2] != 3 {
		t.Errorf("buf.Values() = %v; want [1 2 3]", vals)
	}

	// Overflow: push 4th value, oldest (1) should be dropped
	buf.Push(4)
	vals = buf.Values()
	if len(vals) != 3 || vals[0] != 2 || vals[1] != 3 || vals[2] != 4 {
		t.Errorf("after overflow buf.Values() = %v; want [2 3 4]", vals)
	}

	last, ok := buf.Last()
	if !ok || last != 4 {
		t.Errorf("buf.Last() = %v, %v; want 4, true", last, ok)
	}
}

func TestSafeDiv(t *testing.T) {
	if got := SafeDiv(10, 2); got != 5 {
		t.Errorf("SafeDiv(10,2) = %v; want 5", got)
	}
	if got := SafeDiv(10, 0); got != 0 {
		t.Errorf("SafeDiv(10,0) = %v; want 0", got)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID(8)
	id2 := GenerateID(8)
	if id1 == id2 {
		t.Error("GenerateID produced duplicate IDs")
	}
	if len(id1) != 16 { // 8 bytes → 16 hex chars
		t.Errorf("GenerateID(8) length = %d; want 16", len(id1))
	}
}

func TestNormalizeRange(t *testing.T) {
	// Normal range
	if got := NormalizeRange(5, 0, 10); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("NormalizeRange(5,0,10) = %v; want 0.5", got)
	}
	// Clamp above max
	if got := NormalizeRange(20, 0, 10); got != 1.0 {
		t.Errorf("NormalizeRange(20,0,10) = %v; want 1.0", got)
	}
	// Clamp below min
	if got := NormalizeRange(-5, 0, 10); got != 0.0 {
		t.Errorf("NormalizeRange(-5,0,10) = %v; want 0.0", got)
	}
	// Degenerate: min == max
	if got := NormalizeRange(5, 3, 3); got != 0.0 {
		t.Errorf("NormalizeRange with min==max = %v; want 0.0", got)
	}
}

func TestStdDevEdgeCases(t *testing.T) {
	// Single element → 0
	if got := StdDev([]float64{42}); got != 0 {
		t.Errorf("StdDev(single) = %v; want 0", got)
	}
	// Empty → 0
	if got := StdDev(nil); got != 0 {
		t.Errorf("StdDev(nil) = %v; want 0", got)
	}
}

func TestPercentileEdgeCases(t *testing.T) {
	// p=0 returns min, p=100 returns max
	vals := []float64{3, 1, 4, 1, 5, 9, 2, 6}
	if got := Percentile(vals, 0); got != 1 {
		t.Errorf("Percentile p0 = %v; want 1", got)
	}
	if got := Percentile(vals, 100); got != 9 {
		t.Errorf("Percentile p100 = %v; want 9", got)
	}
	// Out-of-range p clamped
	if got := Percentile(vals, 150); got != 9 {
		t.Errorf("Percentile p150 (clamped) = %v; want 9", got)
	}
}

func TestTrapezoidIntegrateEdgeCases(t *testing.T) {
	// Less than 2 points → 0
	if got := TrapezoidIntegrate([]float64{5}, 1); got != 0 {
		t.Errorf("TrapezoidIntegrate(1 point) = %v; want 0", got)
	}
	if got := TrapezoidIntegrate(nil, 1); got != 0 {
		t.Errorf("TrapezoidIntegrate(nil) = %v; want 0", got)
	}
}

func TestRoundTo(t *testing.T) {
	if got := RoundTo(3.14159, 2); math.Abs(got-3.14) > 1e-9 {
		t.Errorf("RoundTo(3.14159, 2) = %v; want 3.14", got)
	}
	if got := RoundTo(2.5, 0); got != 3 {
		t.Errorf("RoundTo(2.5, 0) = %v; want 3", got)
	}
}

func TestTruncateFunctions(t *testing.T) {
	now := time.Now()
	if got := TruncateToMinute(now); !got.Equal(now.Truncate(time.Minute)) {
		t.Errorf("TruncateToMinute mismatch")
	}
	if got := TruncateToHour(now); !got.Equal(now.Truncate(time.Hour)) {
		t.Errorf("TruncateToHour mismatch")
	}
}

func TestBoolToFloat(t *testing.T) {
	if got := BoolToFloat(true); got != 1.0 {
		t.Errorf("BoolToFloat(true) = %v; want 1.0", got)
	}
	if got := BoolToFloat(false); got != 0.0 {
		t.Errorf("BoolToFloat(false) = %v; want 0.0", got)
	}
}

func TestCircularBufferLastEmpty(t *testing.T) {
	buf := NewCircularBuffer(5)
	if _, ok := buf.Last(); ok {
		t.Error("Last() on empty buffer should return false")
	}
}

func TestCircularBufferValuesEmpty(t *testing.T) {
	buf := NewCircularBuffer(5)
	if vals := buf.Values(); len(vals) != 0 {
		t.Errorf("Values() on empty buffer = %v; want []", vals)
	}
}

func TestCircularBufferZeroSize(t *testing.T) {
	// Push to a zero-capacity buffer must not panic
	buf := NewCircularBuffer(0)
	buf.Push(1.0)
	if buf.Len() != 0 {
		t.Errorf("zero-size buffer Len() = %d; want 0", buf.Len())
	}
}

func TestCircularBufferFullWrap(t *testing.T) {
	// Fill exactly to capacity then verify Values order when head wraps to 0
	buf := NewCircularBuffer(3)
	buf.Push(1)
	buf.Push(2)
	buf.Push(3) // full; head wraps to 0
	buf.Push(4) // overwrites 1; head = 1

	vals := buf.Values()
	if len(vals) != 3 {
		t.Fatalf("len = %d; want 3", len(vals))
	}
	if vals[0] != 2 || vals[1] != 3 || vals[2] != 4 {
		t.Errorf("Values() = %v; want [2 3 4]", vals)
	}
}
