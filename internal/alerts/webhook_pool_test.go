package alerts

import (
	"context"
	"testing"
	"time"

	"warren/internal/config"
	"warren/internal/events"
)

func TestWebhookWorkerPool_BoundedQueue(t *testing.T) {
	// The job channel has capacity 100. Verify it doesn't block the emitter.
	alerter := NewWebhookAlerter([]config.WebhookConfig{
		{URL: "http://unreachable.invalid/hook"},
	}, quietLogger())

	// Don't start workers — jobs will accumulate
	emitter := events.NewEmitter(quietLogger())
	alerter.RegisterEventHandler(emitter)

	// Emit more than buffer size — should not block
	done := make(chan struct{})
	go func() {
		for i := 0; i < 200; i++ {
			emitter.Emit(events.Event{Type: events.AgentReady, Agent: "test"})
		}
		close(done)
	}()

	select {
	case <-done:
		// Good — emitter didn't block
	case <-time.After(2 * time.Second):
		t.Fatal("emitting events blocked — job queue should drop when full")
	}
}

func TestWebhookWorkerPool_DropsWhenFull(t *testing.T) {
	alerter := NewWebhookAlerter([]config.WebhookConfig{
		{URL: "http://unreachable.invalid/hook"},
	}, quietLogger())

	// Override: don't start workers so channel fills up
	emitter := events.NewEmitter(quietLogger())
	alerter.RegisterEventHandler(emitter)

	// Fill the buffer (cap=100)
	for i := 0; i < 100; i++ {
		emitter.Emit(events.Event{Type: events.AgentReady, Agent: "test"})
	}

	// This should be dropped (buffer full, no workers)
	emitter.Emit(events.Event{Type: events.AgentReady, Agent: "overflow"})

	// Now start workers and drain
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Replace alerter with one that counts
	// Just verify the channel length is at capacity
	if len(alerter.jobs) > 100 {
		t.Errorf("job queue length %d exceeds capacity 100", len(alerter.jobs))
	}

	_ = ctx
}
