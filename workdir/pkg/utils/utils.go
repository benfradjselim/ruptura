package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Clamp constrains v within [min, max]
func Clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// NormalizePercent converts a percentage (0-100) to [0,1]
func NormalizePercent(v float64) float64 {
	return Clamp(v/100.0, 0, 1)
}

// NormalizeRange normalizes v from [minVal, maxVal] to [0,1]
func NormalizeRange(v, minVal, maxVal float64) float64 {
	if maxVal == minVal {
		return 0
	}
	return Clamp((v-minVal)/(maxVal-minVal), 0, 1)
}

// Mean computes the arithmetic mean of a slice
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// StdDev computes the standard deviation
func StdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := Mean(values)
	variance := 0.0
	for _, v := range values {
		diff := v - m
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(len(values)))
}

// Percentile computes the p-th percentile (0-100) of a slice.
// p is clamped to [0, 100] to prevent negative index panics.
func Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	p = Clamp(p, 0, 100) // guard against out-of-range p
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	idx := p / 100.0 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo]*(float64(hi)-idx) + sorted[hi]*(idx-float64(lo))
}

// TrapezoidIntegrate numerically integrates values with uniform dt
func TrapezoidIntegrate(values []float64, dt float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sum := 0.0
	for i := 1; i < len(values); i++ {
		sum += (values[i-1] + values[i]) / 2.0 * dt
	}
	return sum
}

// Derivative computes dv/dt for the last two values
func Derivative(prev, curr float64, dt float64) float64 {
	if dt == 0 {
		return 0
	}
	return (curr - prev) / dt
}

// RoundTo rounds a float to n decimal places
func RoundTo(v float64, n int) float64 {
	factor := math.Pow(10, float64(n))
	return math.Round(v*factor) / factor
}

// GenerateID creates a random hex ID of given byte length.
// Panics if the system entropy pool is unavailable (unrecoverable).
func GenerateID(bytes int) string {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(b)
}

// TruncateToMinute truncates a time to the minute
func TruncateToMinute(t time.Time) time.Time {
	return t.Truncate(time.Minute)
}

// TruncateToHour truncates a time to the hour
func TruncateToHour(t time.Time) time.Time {
	return t.Truncate(time.Hour)
}

// BoolToFloat converts a bool to 1.0 or 0.0
func BoolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// SafeDiv divides a by b, returning 0 if b is zero
func SafeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// CircularBuffer is a fixed-size, thread-safe ring buffer for float64.
// All methods are safe for concurrent use.
type CircularBuffer struct {
	mu   sync.RWMutex
	data []float64
	size int
	head int  // next write position
	n    int  // number of valid entries (≤ size)
}

// NewCircularBuffer creates a buffer of given capacity
func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{
		data: make([]float64, size),
		size: size,
	}
}

// Push adds a value, overwriting the oldest when the buffer is full.
func (c *CircularBuffer) Push(v float64) {
	if c.size == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[c.head] = v
	c.head = (c.head + 1) % c.size
	if c.n < c.size {
		c.n++
	}
}

// Values returns all values in insertion order (oldest first).
func (c *CircularBuffer) Values() []float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.n == 0 {
		return nil
	}
	result := make([]float64, c.n)
	if c.n < c.size {
		// Buffer not yet full: valid entries are data[0:n]
		copy(result, c.data[:c.n])
	} else {
		// Buffer full: oldest entry is at head
		copy(result, c.data[c.head:])
		copy(result[c.size-c.head:], c.data[:c.head])
	}
	return result
}

// Len returns the number of stored values.
func (c *CircularBuffer) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.n
}

// Last returns the most recently pushed value.
func (c *CircularBuffer) Last() (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.n == 0 {
		return 0, false
	}
	idx := (c.head - 1 + c.size) % c.size
	return c.data[idx], true
}
