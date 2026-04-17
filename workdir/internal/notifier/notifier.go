// Package notifier delivers OHE alerts to external channels (Slack, webhook).
package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/benfradjselim/ohe/pkg/models"
)

// Channel defines a notification destination persisted in storage.
type Channel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`    // "webhook" | "slack"
	URL     string `json:"url"`     // delivery endpoint
	Enabled bool   `json:"enabled"`
}

// payload is the JSON body sent to webhook and Slack channels.
type payload struct {
	Text        string `json:"text,omitempty"`         // Slack plain-text fallback
	AlertID     string `json:"alert_id,omitempty"`
	AlertName   string `json:"alert_name,omitempty"`
	Host        string `json:"host,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Message     string `json:"message,omitempty"`
	Status      string `json:"status,omitempty"`
	FiredAt     string `json:"fired_at,omitempty"`
}

func buildPayload(a models.Alert) payload {
	text := fmt.Sprintf("[OHE %s] %s on %s — %s", a.Severity, a.Name, a.Host, a.Description)
	return payload{
		Text:      text,
		AlertID:   a.ID,
		AlertName: a.Name,
		Host:      a.Host,
		Severity:  a.Severity,
		Message:   a.Description,
		Status:    a.Status,
		FiredAt:   a.CreatedAt.Format(time.RFC3339),
	}
}

// Dispatcher fans alerts out to all enabled channels.
type Dispatcher struct {
	mu       sync.RWMutex
	channels []Channel
	client   *http.Client
}

// New returns a Dispatcher ready to use.
func New() *Dispatcher {
	return &Dispatcher{
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// SetChannels replaces the active channel list (call after every storage update).
func (d *Dispatcher) SetChannels(channels []Channel) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.channels = channels
}

// Dispatch sends alert a to all enabled channels concurrently.
func (d *Dispatcher) Dispatch(a models.Alert) {
	d.mu.RLock()
	chs := make([]Channel, len(d.channels))
	copy(chs, d.channels)
	d.mu.RUnlock()

	for _, ch := range chs {
		if !ch.Enabled {
			continue
		}
		go func(ch Channel) {
			if err := d.send(ch, a); err != nil {
				log.Printf("[notifier] channel %q (%s) error: %v", ch.Name, ch.Type, err)
			}
		}(ch)
	}
}

func (d *Dispatcher) send(ch Channel, a models.Alert) error {
	p := buildPayload(a)
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	resp, err := d.client.Post(ch.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, ch.URL)
	}
	return nil
}
