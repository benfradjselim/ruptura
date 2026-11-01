package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxEventBody = 16 << 20

// WarningEvent is a Kubernetes warning event about a pod.
type WarningEvent struct {
	Namespace string
	Pod       string
	Reason    string
	At        time.Time
}

type eventList struct {
	Items []struct {
		Reason        string    `json:"reason"`
		LastTimestamp time.Time `json:"lastTimestamp"`
		EventTime     time.Time `json:"eventTime"`
		Series        struct {
			LastObservedTime time.Time `json:"lastObservedTime"`
		} `json:"series"`
		InvolvedObject struct {
			Kind      string `json:"kind"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
		} `json:"involvedObject"`
	} `json:"items"`
}

func parseWarningEvents(body []byte) ([]WarningEvent, error) {
	var list eventList
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("decode events: %w", err)
	}
	out := make([]WarningEvent, 0, len(list.Items))
	for _, item := range list.Items {
		if item.InvolvedObject.Kind != "Pod" {
			continue
		}
		at := item.LastTimestamp
		if item.EventTime.After(at) {
			at = item.EventTime
		}
		if item.Series.LastObservedTime.After(at) {
			at = item.Series.LastObservedTime
		}
		out = append(out, WarningEvent{
			Namespace: item.InvolvedObject.Namespace,
			Pod:       item.InvolvedObject.Name,
			Reason:    item.Reason,
			At:        at,
		})
	}
	return out, nil
}

// ListWarningEvents returns the warning events about pods in the cluster.
func (inf *Informer) ListWarningEvents(ctx context.Context) ([]WarningEvent, error) {
	url := inf.apiBase + "/api/v1/events?fieldSelector=type%3DWarning&limit=1000"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+inf.token)
	resp, err := inf.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxEventBody))
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list events: status %d", resp.StatusCode)
	}
	return parseWarningEvents(body)
}
