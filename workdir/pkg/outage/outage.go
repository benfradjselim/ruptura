// Package outage turns repeated failures of a dependency into two log lines:
// one when it stops answering and one when it answers again.
package outage

import (
	"sync"
	"time"
)

// Log reports the outages of one dependency. It is safe for concurrent use.
type Log struct {
	name string
	logf func(format string, args ...interface{})
	now  func() time.Time

	mu       sync.Mutex
	since    time.Time
	failures int
}

// New returns a Log for the dependency name that writes with logf.
func New(name string, logf func(format string, args ...interface{})) *Log {
	return &Log{name: name, logf: logf, now: time.Now}
}

// Failed records a failed call. Only the first failure of an outage is logged,
// with its cause.
func (l *Log) Failed(what string, err error) {
	l.mu.Lock()
	first := l.since.IsZero()
	if first {
		l.since = l.now()
	}
	l.failures++
	l.mu.Unlock()
	if first {
		l.logf("%s unreachable (%s: %v), retrying quietly until it answers", l.name, what, err)
	}
}

// Succeeded records a successful call and, after an outage, logs how long it
// lasted and how many calls failed.
func (l *Log) Succeeded() {
	l.mu.Lock()
	since, failures := l.since, l.failures
	l.since, l.failures = time.Time{}, 0
	l.mu.Unlock()
	if !since.IsZero() {
		l.logf("%s reachable again after %s, %d failed calls", l.name, l.now().Sub(since).Round(time.Second), failures)
	}
}
