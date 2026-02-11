package alerts

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"warren/internal/config"
	"warren/internal/events"
)

// WebhookAlerter sends event notifications to configured webhook URLs.
type WebhookAlerter struct {
	configs []config.WebhookConfig
	client  *http.Client
	logger  *slog.Logger
}

// NewWebhookAlerter creates a new webhook alerter.
func NewWebhookAlerter(configs []config.WebhookConfig, logger *slog.Logger) *WebhookAlerter {
	return &WebhookAlerter{
		configs: configs,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.With("component", "webhook-alerter"),
	}
}

// RegisterEventHandler registers the alerter as an event handler on the emitter.
func (w *WebhookAlerter) RegisterEventHandler(emitter *events.Emitter) {
	emitter.OnEvent(func(ev events.Event) {
		for _, cfg := range w.configs {
			if w.matches(cfg, ev.Type) {
				go w.send(cfg, ev)
			}
		}
	})
}

func (w *WebhookAlerter) matches(cfg config.WebhookConfig, eventType string) bool {
	if len(cfg.Events) == 0 {
		return true // no filter = all events
	}
	for _, e := range cfg.Events {
		if e == eventType {
			return true
		}
	}
	return false
}

func (w *WebhookAlerter) send(cfg config.WebhookConfig, ev events.Event) {
	body, err := json.Marshal(ev)
	if err != nil {
		w.logger.Error("webhook: failed to marshal event", "error", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		w.logger.Error("webhook: failed to create request", "error", err, "url", cfg.URL)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Error("webhook: request failed", "error", err, "url", cfg.URL)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		w.logger.Warn("webhook: non-success status", "status", resp.StatusCode, "url", cfg.URL)
	}
}
