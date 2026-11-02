package failuremodes

import (
	"sort"
	"time"
)

// trackRecordMinLead is the shortest warning that counts as useful.
const trackRecordMinLead = 5 * time.Minute

// maxTrackRecord bounds the outcomes kept in memory.
const maxTrackRecord = 2048

type failureOutcome struct {
	at   time.Time
	lead time.Duration
}

// TrackRecord summarizes how early failures were announced in a period.
type TrackRecord struct {
	Failures   int
	Warned     int
	MedianLead time.Duration
}

// NoteForecast records that a failure of workload is currently forecast. The first
// note of a cycle is the time of the warning.
func (t *Tracker) NoteForecast(workload string, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.forecasts[workload]; !ok {
		t.forecasts[workload] = at
	}
}

func (t *Tracker) recordOutcome(workload string, at time.Time) {
	outcome := failureOutcome{at: at}
	if warned, ok := t.forecasts[workload]; ok {
		outcome.lead = at.Sub(warned)
		delete(t.forecasts, workload)
	}
	t.appendOutcome(outcome)
	if t.outcomeRecorder != nil {
		t.outcomeRecorder(outcome.at, outcome.lead)
	}
}

func (t *Tracker) appendOutcome(outcome failureOutcome) {
	t.outcomes = append(t.outcomes, outcome)
	if len(t.outcomes) > maxTrackRecord {
		t.outcomes = t.outcomes[len(t.outcomes)-maxTrackRecord:]
	}
}

// SetOutcomeRecorder sets a callback that receives each failure and its warning.
func (t *Tracker) SetOutcomeRecorder(recorder func(at time.Time, lead time.Duration)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.outcomeRecorder = recorder
}

// RestoreOutcome loads one outcome saved by an earlier run.
func (t *Tracker) RestoreOutcome(at time.Time, lead time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.appendOutcome(failureOutcome{at: at, lead: lead})
}

// TrackRecord reports failures between from and to, and how many were warned at
// least trackRecordMinLead ahead.
func (t *Tracker) TrackRecord(from, to time.Time) TrackRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var record TrackRecord
	var leads []time.Duration
	for _, outcome := range t.outcomes {
		if outcome.at.Before(from) || outcome.at.After(to) {
			continue
		}
		record.Failures++
		if outcome.lead >= trackRecordMinLead {
			record.Warned++
			leads = append(leads, outcome.lead)
		}
	}
	if len(leads) > 0 {
		sort.Slice(leads, func(i, j int) bool { return leads[i] < leads[j] })
		record.MedianLead = leads[len(leads)/2]
	}
	return record
}
