package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidate_InvalidHostnameRejected(t *testing.T) {
	cfg := &Config{Agents: map[string]*Agent{
		"a": {
			Hostname: "-invalid.com",
			Backend:  "http://x",
			Policy:   "unmanaged",
		},
	}}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid hostname")
	}
	if !strings.Contains(err.Error(), "hostname") {
		t.Errorf("error = %v, want hostname validation error", err)
	}
}

func TestValidate_InvalidAdditionalHostnameRejected(t *testing.T) {
	cfg := &Config{Agents: map[string]*Agent{
		"a": {
			Hostname:  "good.com",
			Hostnames: []string{"bad_host.com"},
			Backend:   "http://x",
			Policy:    "unmanaged",
		},
	}}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected error for invalid additional hostname")
	}
}

func TestValidate_InvalidHealthURLSchemeRejected(t *testing.T) {
	cfg := &Config{Agents: map[string]*Agent{
		"a": {
			Hostname:  "a.com",
			Backend:   "http://x",
			Policy:    "always-on",
			Container: Container{Name: "svc"},
			Health:    Health{URL: "ftp://x/health"},
		},
	}}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected error for ftp health URL")
	}
	if !strings.Contains(err.Error(), "health URL") {
		t.Errorf("error = %v, want health URL error", err)
	}
}

func TestValidate_WebhookURLValidated(t *testing.T) {
	cfg := &Config{
		Agents: map[string]*Agent{
			"a": {Hostname: "a.com", Backend: "http://x", Policy: "unmanaged"},
		},
		Webhooks: []WebhookConfig{
			{URL: "file:///etc/passwd"},
		},
	}
	err := validate(cfg)
	if err == nil {
		t.Fatal("expected error for file:// webhook URL")
	}
	if !strings.Contains(err.Error(), "webhook") {
		t.Errorf("error = %v, want webhook error", err)
	}
}

func TestValidate_WakeCooldownDefault(t *testing.T) {
	cfg := &Config{Agents: map[string]*Agent{
		"a": {
			Hostname:  "a.com",
			Backend:   "http://x",
			Policy:    "on-demand",
			Container: Container{Name: "svc"},
			Health:    Health{URL: "http://x/h"},
			Idle:      IdleConfig{Timeout: time.Minute},
		},
	}}
	applyDefaults(cfg)
	agent := cfg.Agents["a"]
	if agent.Idle.WakeCooldown != 30*time.Second {
		t.Errorf("WakeCooldown = %v, want 30s default", agent.Idle.WakeCooldown)
	}
}
