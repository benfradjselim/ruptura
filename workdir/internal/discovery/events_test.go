package discovery

import (
	"testing"
	"time"
)

func TestParseWarningEvents(t *testing.T) {
	body := []byte(`{"items":[
	 {"reason":"FailedMount","type":"Warning","lastTimestamp":"2026-11-01T10:00:00Z","involvedObject":{"kind":"Pod","namespace":"n","name":"p"}},
	 {"reason":"BackOff","type":"Warning","eventTime":"2026-11-01T11:00:00.000000Z","series":{"lastObservedTime":"2026-11-01T12:00:00.000000Z"},"involvedObject":{"kind":"Pod","namespace":"n","name":"q"}},
	 {"reason":"Scaled","type":"Warning","lastTimestamp":"2026-11-01T10:00:00Z","involvedObject":{"kind":"Deployment","namespace":"n","name":"d"}}
	]}`)
	got, err := parseWarningEvents(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("events = %+v, want 2 pod events", got)
	}
	if got[0].Pod != "p" || got[0].Reason != "FailedMount" || !got[0].At.Equal(time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("first = %+v", got[0])
	}
	if !got[1].At.Equal(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("series time not used: %+v", got[1])
	}
	if _, err := parseWarningEvents([]byte("not json")); err == nil {
		t.Error("invalid body must return an error")
	}
}
