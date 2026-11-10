package discovery

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestStreamWatch(t *testing.T) {
	tests := []struct {
		name    string
		stream  string
		wantRV  string
		wantErr error
		handled int
	}{
		{
			name:    "server timeout ends cleanly at the last version",
			stream:  `{"type":"ADDED","object":{"metadata":{"resourceVersion":"11"}}}{"type":"MODIFIED","object":{"metadata":{"resourceVersion":"12"}}}`,
			wantRV:  "12",
			handled: 2,
		},
		{
			name:   "bookmark advances the version without an event",
			stream: `{"type":"BOOKMARK","object":{"metadata":{"resourceVersion":"40"}}}`,
			wantRV: "40",
		},
		{
			name:    "expired version asks for a list",
			stream:  `{"type":"ADDED","object":{"metadata":{"resourceVersion":"11"}}}{"type":"ERROR","object":{"code":410}}`,
			wantRV:  "11",
			wantErr: errWatchExpired,
			handled: 1,
		},
		{
			name:   "empty stream keeps the starting version",
			stream: ``,
			wantRV: "10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled := 0
			rv, err := streamWatch(strings.NewReader(tt.stream), "10", func(_ string, object json.RawMessage) string {
				handled++
				var p k8sPod
				_ = json.Unmarshal(object, &p)
				return p.Metadata.ResourceVersion
			})
			if rv != tt.wantRV || !errors.Is(err, tt.wantErr) || handled != tt.handled {
				t.Fatalf("rv=%q err=%v handled=%d, want rv=%q err=%v handled=%d", rv, err, handled, tt.wantRV, tt.wantErr, tt.handled)
			}
		})
	}
}

func TestStreamWatch_TruncatedEventIsAnError(t *testing.T) {
	_, err := streamWatch(strings.NewReader(`{"type":"ADDED","object":{"metad`), "10", func(string, json.RawMessage) string { return "" })
	if err == nil || errors.Is(err, errWatchExpired) {
		t.Fatalf("err = %v, want a decode error", err)
	}
}
