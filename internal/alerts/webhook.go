package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"warren/internal/config"
	"warren/internal/events"
)

type webhookJob struct {
	cfg config.WebhookConfig
	ev  events.Event
}

// WebhookAlerter sends event notifications to configured webhook URLs.
type WebhookAlerter struct {
	configs []config.WebhookConfig
	client  *http.Client
	logger  *slog.Logger
	jobs    chan webhookJob
}

// NewWebhookAlerter creates a new webhook alerter.
func NewWebhookAlerter(configs []config.WebhookConfig, logger *slog.Logger) *WebhookAlerter {
	return &WebhookAlerter{
		configs: configs,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.With("component", "webhook-alerter"),
		jobs:   make(chan webhookJob, 100),
	}
}

// Start launches the worker pool. Call this before registering event handlers.
func (w *WebhookAlerter) Start(ctx context.Context) {
	const numWorkers = 5
	for i := 0; i < numWorkers; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-w.jobs:
					w.send(job.cfg, job.ev)
				}
			}
		}()
	}
}

// RegisterEventHandler registers the alerter as an event handler on the emitter.
func (w *WebhookAlerter) RegisterEventHandler(emitter *events.Emitter) {
	emitter.OnEvent(func(ev events.Event) {
		for _, cfg := range w.configs {
			if w.matches(cfg, ev.Type) {
				select {
				case w.jobs <- webhookJob{cfg: cfg, ev: ev}:
				default:
					w.logger.Warn("webhook job queue full, dropping event", "event", ev.Type, "url", cfg.URL)
				}
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
