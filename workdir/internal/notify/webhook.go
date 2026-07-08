// Package notify delivers outbound webhook notifications for Tier-2+ rupture
// events. Community edition supports Slack (incoming webhook) and a generic
// JSON webhook. PagerDuty/OpsGenie routing and Tier-1 auto-execution stay in
// the paid autopilot edition — see CLAUDE.md's edition boundary: community
// fires alerts, autopilot acts on them.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TierLevel matches the action engine tier constants.
type TierLevel int

const (
	Tier1 TierLevel = 1
	Tier2 TierLevel = 2
	Tier3 TierLevel = 3
)

// Event describes a rupture/action event that should be notified.
type Event struct {
	ID         string
	Host       string
	ActionType string // "scale", "restart", "alert", "notify", "page", "custom"
	Tier       TierLevel
	Reason     string
	Timestamp  time.Time
}

// Config holds destination configuration for outbound notifications.
type Config struct {
	// SlackWebhookURL is an incoming webhook URL from api.slack.com/apps.
	// Leave empty to disable Slack notifications.
	SlackWebhookURL string

	// GenericWebhookURL receives a JSON payload for any custom destination.
	// Leave empty to disable the generic webhook.
	GenericWebhookURL string

	// MinTier is the minimum tier level that triggers a notification (default 2).
	MinTier TierLevel
}

// Notifier sends rupture events to configured destinations.
type Notifier struct {
	cfg    Config
	client *http.Client
}

// New creates a Notifier. Pass a zero Config to create a no-op notifier.
func New(cfg Config) *Notifier {
	if cfg.MinTier == 0 {
		cfg.MinTier = Tier2
	}
	return &Notifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify delivers the event to all configured destinations.
// Errors from individual destinations are logged but do not block each other.
// Returns a combined error string if any destination failed.
func (n *Notifier) Notify(ctx context.Context, ev Event) error {
	if ev.Tier < n.cfg.MinTier {
		return nil
	}
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}

	var errs []string

	if n.cfg.SlackWebhookURL != "" {
		if err := n.sendSlack(ctx, ev); err != nil {
			errs = append(errs, fmt.Sprintf("slack: %v", err))
		}
	}
	if n.cfg.GenericWebhookURL != "" {
		if err := n.sendGeneric(ctx, ev); err != nil {
			errs = append(errs, fmt.Sprintf("webhook: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notify: %s", strings.Join(errs, "; "))
	}
	return nil
}

// sendSlack posts an Incoming Webhook message.
func (n *Notifier) sendSlack(ctx context.Context, ev Event) error {
	emoji := tierEmoji(ev.Tier)
	text := fmt.Sprintf("%s *[Ruptura]* %s on `%s`\n>Tier %d · %s\n>_%s_",
		emoji, strings.ToUpper(ev.ActionType), ev.Host, ev.Tier, ev.Timestamp.UTC().Format("15:04 UTC"), ev.Reason)

	payload := map[string]any{
		"text": text,
		"attachments": []map[string]any{{
			"color": tierColor(ev.Tier),
			"fields": []map[string]any{
				{"title": "Workload", "value": ev.Host, "short": true},
				{"title": "Action", "value": ev.ActionType, "short": true},
				{"title": "Tier", "value": fmt.Sprintf("%d", ev.Tier), "short": true},
				{"title": "Event ID", "value": ev.ID, "short": true},
			},
			"footer": "ruptura",
			"ts":     ev.Timestamp.Unix(),
		}},
	}
	return n.post(ctx, n.cfg.SlackWebhookURL, payload)
}

// sendGeneric posts a standard JSON envelope to any HTTP endpoint.
func (n *Notifier) sendGeneric(ctx context.Context, ev Event) error {
	payload := map[string]any{
		"source":      "ruptura",
		"event":       "rupture.recommended",
		"event_id":    ev.ID,
		"workload":    ev.Host,
		"action_type": ev.ActionType,
		"tier":        ev.Tier,
		"reason":      ev.Reason,
		"timestamp":   ev.Timestamp.UTC().Format(time.RFC3339),
	}
	return n.post(ctx, n.cfg.GenericWebhookURL, payload)
}

func (n *Notifier) post(ctx context.Context, url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		// err may embed the URL (net/http wraps it in *url.Error) — never let
		// that reach a caller that logs it wholesale without redaction.
		return fmt.Errorf("post failed: %w", redactURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

// redactURLError strips the destination URL out of net/http's *url.Error
// (which otherwise embeds the full request URL — including any path-based
// tokens some webhook providers use — in its Error() string), so callers
// that log post() failures never leak webhook secrets.
func redactURLError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("request error: %s", classifyNetError(err))
}

func classifyNetError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "connection refused"):
		return "connection refused"
	case strings.Contains(msg, "no such host"):
		return "dns lookup failed"
	default:
		return "network error"
	}
}

func tierEmoji(t TierLevel) string {
	switch t {
	case Tier3:
		return ":rotating_light:"
	case Tier2:
		return ":warning:"
	default:
		return ":information_source:"
	}
}

func tierColor(t TierLevel) string {
	switch t {
	case Tier3:
		return "#dc2626"
	case Tier2:
		return "#d97706"
	default:
		return "#2563eb"
	}
}

// Config returns the current notifier configuration.
func (n *Notifier) Config() Config {
	return n.cfg
}

// Configured reports whether at least one destination is set.
func (n *Notifier) Configured() bool {
	return n.cfg.SlackWebhookURL != "" || n.cfg.GenericWebhookURL != ""
}

// NotifyChannel sends a test event to a single named channel, bypassing the
// MinTier gate — used for "send a test notification" from the settings UI.
// channel must be one of: "slack", "webhook".
func (n *Notifier) NotifyChannel(ctx context.Context, channel string, ev Event) error {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	switch channel {
	case "slack":
		if n.cfg.SlackWebhookURL == "" {
			return fmt.Errorf("slack not configured")
		}
		return n.sendSlack(ctx, ev)
	case "webhook":
		if n.cfg.GenericWebhookURL == "" {
			return fmt.Errorf("webhook not configured")
		}
		return n.sendGeneric(ctx, ev)
	default:
		return fmt.Errorf("unknown channel %q — must be slack or webhook", channel)
	}
}
