package services

import (
	"strings"
	"testing"
)

func TestRegistry_CachedReverseProxy(t *testing.T) {
	r := testRegistry()
	err := r.Register("app.example.com", "http://localhost:3000", "agent-a")
	if err != nil {
		t.Fatal(err)
	}

	svc, ok := r.Lookup("app.example.com")
	if !ok {
		t.Fatal("expected lookup to succeed")
	}

	// Proxy should be created at registration time
	if svc.Proxy == nil {
		t.Fatal("expected Proxy to be non-nil (cached at registration)")
	}
	if svc.TargetURL == nil {
		t.Fatal("expected TargetURL to be non-nil")
	}

	// Re-lookup should return same proxy instance
	svc2, _ := r.Lookup("app.example.com")
	if svc.Proxy != svc2.Proxy {
		t.Error("expected same Proxy instance on repeated lookup (cached)")
	}
}

func TestRegistry_RejectsInvalidHostname(t *testing.T) {
	r := testRegistry()

	invalids := []string{
		"",
		"-bad.com",
		"bad-.com",
		"under_score.com",
		"has space.com",
	}
	for _, h := range invalids {
		err := r.Register(h, "http://localhost:3000", "agent")
		if err == nil {
			t.Errorf("Register(%q) = nil, want error for invalid hostname", h)
		} else if !strings.Contains(err.Error(), "invalid hostname") {
			t.Errorf("Register(%q) error = %v, want 'invalid hostname'", h, err)
		}
	}
}

func TestRegistry_RejectsReservedHostname(t *testing.T) {
	r := testRegistry()
	r.ReserveHostname("reserved.example.com")

	err := r.Register("reserved.example.com", "http://localhost:3000", "agent")
	if err == nil {
		t.Error("expected error for reserved hostname")
	} else if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("error = %v, want 'reserved'", err)
	}
}

func TestRegistry_RejectsUnsafeTargets(t *testing.T) {
	r := testRegistry()

	unsafe := []struct {
		target  string
		wantErr string
	}{
		{"file:///etc/passwd", "scheme"},
		{"ftp://evil.com/x", "scheme"},
		{"http://169.254.169.254/latest", "blocked"},
		{"http://metadata.google.internal/v1", "blocked"},
		{"unix:///var/run/docker.sock", "scheme"},
	}
	for _, tt := range unsafe {
		err := r.Register("test.example.com", tt.target, "agent")
		if err == nil {
			t.Errorf("Register(target=%q) = nil, want error containing %q", tt.target, tt.wantErr)
		} else if !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("Register(target=%q) = %v, want error containing %q", tt.target, err, tt.wantErr)
		}
		// Clean up for next iteration
		r.Deregister("test.example.com")
	}
}

func TestRegistry_ValidTargetAccepted(t *testing.T) {
	r := testRegistry()
	err := r.Register("valid.example.com", "http://10.0.0.5:3000", "agent")
	if err != nil {
		t.Errorf("valid local target rejected: %v", err)
	}
}
