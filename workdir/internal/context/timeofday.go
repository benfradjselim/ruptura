package context

import "time"

// TimeOfDayManager tracks 24 hourly buckets.
// BucketOf returns the bucket index (0-23) for a given time.
type TimeOfDayManager struct {
    buckets [24]float64 // moving average of baseline per hour
    lambda  float64     // ELS lambda for baseline update (default 0.99)
}

func NewTimeOfDayManager() *TimeOfDayManager {
    return &TimeOfDayManager{lambda: 0.99}
}
func (m *TimeOfDayManager) BucketOf(t time.Time) int { return t.Hour() }
func (m *TimeOfDayManager) Update(t time.Time, value float64) {
    b := t.Hour()
    m.buckets[b] = m.lambda*m.buckets[b] + (1-m.lambda)*value
}
func (m *TimeOfDayManager) Baseline(t time.Time) float64 { return m.buckets[t.Hour()] }
