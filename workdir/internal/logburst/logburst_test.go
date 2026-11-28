package logburst

import (
	"testing"
	"time"
)

var start = time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)

func TestIsError(t *testing.T) {
	tests := []struct {
		number int
		text   string
		want   bool
	}{
		{17, "", true},
		{21, "FATAL", true},
		{9, "INFO", false},
		{13, "WARN", false},
		{0, "error", true},
		{0, "Critical", true},
		{0, "info", false},
		{9, "ERROR", false}, // the number wins when set
	}
	for _, tt := range tests {
		if got := IsError(tt.number, tt.text); got != tt.want {
			t.Errorf("IsError(%d, %q) = %v, want %v", tt.number, tt.text, got, tt.want)
		}
	}
}

// feed adds perMinute errors for each minute in [from, to).
func feed(c *Counter, workload string, from, to int, perMinute int) {
	for m := from; m < to; m++ {
		for i := 0; i < perMinute; i++ {
			c.Add(workload, start.Add(time.Duration(m)*time.Minute+time.Duration(i)*time.Second))
		}
	}
}

func TestReadings(t *testing.T) {
	c := New()
	feed(c, "shop/Deployment/api", 0, 55, 4)   // usual: 4 errors a minute
	feed(c, "shop/Deployment/api", 55, 60, 40) // burst: 40 a minute
	feed(c, "shop/Deployment/web", 0, 60, 20)  // steady and busy
	feed(c, "shop/Deployment/new", 50, 60, 50) // too little history
	feed(c, "shop/Deployment/quiet", 0, 55, 1)
	feed(c, "shop/Deployment/quiet", 55, 60, 3) // rise below the floor
	got := map[string]float64{}
	for _, r := range c.Readings(start.Add(60 * time.Minute)) {
		got[r.Workload] = r.Ratio
	}
	if got["shop/Deployment/api"] != 10 {
		t.Fatalf("api ratio = %v, want 10 (40 against a median of 4)", got["shop/Deployment/api"])
	}
	if got["shop/Deployment/web"] != 1 {
		t.Fatalf("web ratio = %v, want 1", got["shop/Deployment/web"])
	}
	if _, ok := got["shop/Deployment/new"]; ok {
		t.Fatal("reported a workload with too little history")
	}
	if ratio, ok := got["shop/Deployment/quiet"]; !ok || ratio != 0 {
		t.Fatalf("quiet ratio = %v, want 0 below the floor", got["shop/Deployment/quiet"])
	}
}

func TestOldMinutesForgotten(t *testing.T) {
	c := New()
	feed(c, "shop/Deployment/api", 0, 60, 4)
	if got := c.Readings(start.Add(3 * time.Hour)); len(got) != 0 {
		t.Fatalf("readings after two silent hours = %+v", got)
	}
}
