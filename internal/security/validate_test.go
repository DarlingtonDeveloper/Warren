package security

import (
	"strings"
	"testing"
)

func TestValidateHostname(t *testing.T) {
	valid := []string{"example.com", "a.b.c", "my-host", "a", "foo-bar.baz"}
	for _, h := range valid {
		if err := ValidateHostname(h); err != nil {
			t.Errorf("ValidateHostname(%q) = %v, want nil", h, err)
		}
	}

	invalid := []struct {
		host    string
		wantErr string
	}{
		{"", "empty"},
		{"host name.com", "invalid characters"},
		{"-bad.com", "invalid characters"},
		{"bad-.com", "invalid characters"},
		{strings.Repeat("a", 64) + ".com", "exceeds 63"},
		{strings.Repeat("a.", 128), "exceeds 253"},
		{"foo..bar", "empty label"},
		{"under_score.com", "invalid characters"},
	}
	for _, tt := range invalid {
		if err := ValidateHostname(tt.host); err == nil {
			t.Errorf("ValidateHostname(%q) = nil, want error containing %q", tt.host, tt.wantErr)
		} else if !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("ValidateHostname(%q) = %v, want error containing %q", tt.host, err, tt.wantErr)
		}
	}
}

func TestValidateWebhookURL(t *testing.T) {
	// Valid public URLs should pass.
	if err := ValidateWebhookURL("https://hooks.slack.com/foo"); err != nil {
		t.Errorf("unexpected error for public URL: %v", err)
	}

	// Private IPs should be rejected.
	rejectURLs := []string{
		"http://127.0.0.1/hook",
		"http://10.0.0.1/hook",
		"http://192.168.1.1/hook",
		"http://172.16.0.1/hook",
		"http://169.254.169.254/latest",
		"ftp://example.com/file",
		"file:///etc/passwd",
	}
	for _, u := range rejectURLs {
		if err := ValidateWebhookURL(u); err == nil {
			t.Errorf("ValidateWebhookURL(%q) = nil, want error", u)
		}
	}
}

func TestValidateHealthURL(t *testing.T) {
	// Private IPs are allowed for health checks.
	valid := []string{
		"http://10.0.0.1:8080/health",
		"http://localhost:8080/health",
		"https://my-container:443/health",
	}
	for _, u := range valid {
		if err := ValidateHealthURL(u); err != nil {
			t.Errorf("ValidateHealthURL(%q) = %v, want nil", u, err)
		}
	}

	// Bad schemes should be rejected.
	invalid := []string{
		"ftp://10.0.0.1/health",
		"file:///etc/passwd",
	}
	for _, u := range invalid {
		if err := ValidateHealthURL(u); err == nil {
			t.Errorf("ValidateHealthURL(%q) = nil, want error", u)
		}
	}
}
