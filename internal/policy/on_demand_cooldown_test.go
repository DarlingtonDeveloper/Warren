package policy

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"warren/internal/events"
)

func TestOnDemandWakeCooldown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	emitter := events.NewEmitter(logger)
	activity := newMockActivity()
	ws := &mockWSSource{}

	mgr := &mockLifecycle{status: "exited"}
	od := NewOnDemand(mgr, OnDemandConfig{
		Agent:              "test",
		ContainerName:      "test-svc",
		HealthURL:          srv.URL,
		Hostname:           "test.com",
		CheckInterval:      50 * time.Millisecond,
		StartupTimeout:     5 * time.Second,
		IdleTimeout:        150 * time.Millisecond,
		WakeCooldown:       1 * time.Second,
		MaxFailures:        3,
		MaxRestartAttempts: 2,
	}, activity, ws, emitter, logger)
	od.SetInitialState(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go od.Start(ctx)

	// Wait for sleeping state — need to wait longer than cooldown since
	// setState("sleeping") sets lastSleepTime on init
	time.Sleep(1100 * time.Millisecond)

	// Wake it up
	od.OnRequest()
	deadline := time.After(3 * time.Second)
	for od.State() != "ready" {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for ready, state = %q", od.State())
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	// Wait for idle → sleeping
	deadline = time.After(3 * time.Second)
	for od.State() != "sleeping" {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for sleep, state = %q", od.State())
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	startCount := atomic.LoadInt32(&mgr.startCalled)

	// Try to wake immediately — should be ignored due to cooldown
	od.OnRequest()
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&mgr.startCalled) != startCount {
		t.Error("wake during cooldown should not trigger container start")
	}
	if od.State() != "sleeping" {
		t.Errorf("state = %q, want sleeping (cooldown active)", od.State())
	}

	// Wait for cooldown to expire (1s total, already waited ~100ms + some overhead)
	time.Sleep(1 * time.Second)
	od.OnRequest()

	deadline = time.After(3 * time.Second)
	for od.State() != "ready" {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for ready after cooldown, state = %q", od.State())
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func TestOnDemandNoCooldownWhenZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	emitter := events.NewEmitter(logger)
	activity := newMockActivity()
	ws := &mockWSSource{}

	mgr := &mockLifecycle{status: "exited"}
	od := NewOnDemand(mgr, OnDemandConfig{
		Agent:              "test",
		ContainerName:      "test-svc",
		HealthURL:          srv.URL,
		Hostname:           "test.com",
		CheckInterval:      50 * time.Millisecond,
		StartupTimeout:     5 * time.Second,
		IdleTimeout:        150 * time.Millisecond,
		WakeCooldown:       0, // no cooldown
		MaxFailures:        3,
		MaxRestartAttempts: 2,
	}, activity, ws, emitter, logger)
	od.SetInitialState(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go od.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	// Wake → ready → idle → sleeping → immediate wake should work
	od.OnRequest()
	deadline := time.After(3 * time.Second)
	for od.State() != "ready" {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for ready")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	deadline = time.After(3 * time.Second)
	for od.State() != "sleeping" {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for sleep")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	// Immediate re-wake with zero cooldown should succeed
	od.OnRequest()
	deadline = time.After(3 * time.Second)
	for od.State() != "ready" {
		select {
		case <-deadline:
			t.Fatalf("timed out: zero cooldown should allow immediate re-wake, state = %q", od.State())
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
}
