package events

import (
	"fmt"
	"sync"
	"time"
)

const maxEvents = 300

type Severity string

const (
	SevInfo     Severity = "info"
	SevWarning  Severity = "warning"
	SevCritical Severity = "critical"
	SevEmergency Severity = "emergency"
)

type Event struct {
	ID       string    `json:"id"`
	TS       time.Time `json:"ts"`
	Workload string    `json:"workload"`
	Type     string    `json:"type"`
	Severity Severity  `json:"severity"`
	Message  string    `json:"message"`
	FusedR   float64   `json:"fused_r,omitempty"`
}

type Bus struct {
	mu     sync.RWMutex
	events []Event
	seq    int
	// track last known FusedR per workload to detect threshold crossings
	lastFusedR  map[string]float64
	subscribers map[chan Event]struct{}
}

func New() *Bus {
	return &Bus{
		lastFusedR:  make(map[string]float64),
		subscribers: make(map[chan Event]struct{}),
	}
}

// Subscribe returns a buffered channel that receives every future event.
// Call Unsubscribe when the consumer is done to avoid leaking the channel.
func (b *Bus) Subscribe() chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes the channel from the fan-out list and closes it.
func (b *Bus) Unsubscribe(ch chan Event) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *Bus) Push(ev Event) {
	b.mu.Lock()
	b.seq++
	ev.ID = fmt.Sprintf("ev_%06d", b.seq)
	if ev.TS.IsZero() {
		ev.TS = time.Now()
	}
	b.events = append(b.events, ev)
	if len(b.events) > maxEvents {
		b.events = b.events[len(b.events)-maxEvents:]
	}
	// Fan-out to SSE subscribers — non-blocking; drop if subscriber is slow.
	for ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
	b.mu.Unlock()
}

// ObserveFusedR detects threshold crossings and auto-emits events.
func (b *Bus) ObserveFusedR(workload string, fusedR float64) {
	b.mu.Lock()
	prev := b.lastFusedR[workload]
	b.lastFusedR[workload] = fusedR
	b.mu.Unlock()

	type transition struct {
		low, high float64
		sev       Severity
		evType    string
		msg       string
	}
	transitions := []transition{
		{0, 1.5, SevInfo, "recovered", fmt.Sprintf("%s recovered — FusedR %.2f", workload, fusedR)},
		{1.5, 3.0, SevWarning, "warning", fmt.Sprintf("%s entered warning — FusedR %.2f", workload, fusedR)},
		{3.0, 5.0, SevCritical, "critical", fmt.Sprintf("%s entered critical — FusedR %.2f", workload, fusedR)},
		{5.0, 99, SevEmergency, "emergency", fmt.Sprintf("%s EMERGENCY — FusedR %.2f", workload, fusedR)},
	}

	var sev Severity
	var evType, msg string
	for _, tr := range transitions {
		if fusedR >= tr.low && fusedR < tr.high {
			if prev < tr.low || prev >= tr.high {
				sev = tr.sev
				evType = tr.evType
				msg = tr.msg
			}
			break
		}
	}
	if evType == "" {
		return
	}
	b.Push(Event{
		TS:       time.Now(),
		Workload: workload,
		Type:     evType,
		Severity: sev,
		Message:  msg,
		FusedR:   fusedR,
	})
}

// Recent returns the n most recent events (newest first).
func (b *Bus) Recent(n int) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	total := len(b.events)
	if n > total {
		n = total
	}
	out := make([]Event, n)
	for i := 0; i < n; i++ {
		out[i] = b.events[total-1-i]
	}
	return out
}
