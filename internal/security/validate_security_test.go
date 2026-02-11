package security

import (
	"strings"
	"testing"
)

func TestValidateHostname_Comprehensive(t *testing.T) {
	valid := []string{
		"example.com",
		"a.b.c.d.e",
		"my-host",
		"a",
		"foo-bar.baz",
		"123.456",
		"a1.b2.c3",
		strings.Repeat("a", 63) + ".com", // max label length
	}
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
		{strings.Repeat("a", 254), "exceeds 253"},
		{"foo..bar", "empty label"},
		{"under_score.com", "invalid characters"},
		{".leading.dot", "empty label"},
		{"trailing.dot.", "empty label"},
		{"sp ace", "invalid characters"},
		{"ex!ample.com", "invalid characters"},
		{"ex@mple.com", "invalid characters"},
	}
	for _, tt := range invalid {
		t.Run(tt.host, func(t *testing.T) {
			err := ValidateHostname(tt.host)
			if err == nil {
				t.Errorf("ValidateHostname(%q) = nil, want error containing %q", tt.host, tt.wantErr)
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ValidateHostname(%q) = %v, want error containing %q", tt.host, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTargetURL_PrivateIPsRejected(t *testing.T) {
	rejected := []string{
		"http://127.0.0.1/hook",
		"http://10.0.0.1/hook",
		"http://10.255.255.255/x",
		"http://192.168.1.1/hook",
		"http://192.168.0.0/hook",
		"http://172.16.0.1/hook",
		"http://172.31.255.255/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://[::1]/hook",
	}
	for _, u := range rejected {
		if err := ValidateWebhookURL(u); err == nil {
			t.Errorf("ValidateWebhookURL(%q) = nil, want error (private IP)", u)
		}
	}
}

func TestValidateTargetURL_LoopbackRejected(t *testing.T) {
	loopbacks := []string{
		"http://127.0.0.1/x",
		"http://127.0.0.2/x",
	}
	for _, u := range loopbacks {
		err := ValidateWebhookURL(u)
		if err == nil {
			t.Errorf("ValidateWebhookURL(%q) = nil, want loopback error", u)
		} else if !strings.Contains(err.Error(), "loopback") {
			t.Errorf("ValidateWebhookURL(%q) = %v, want loopback error", u, err)
		}
	}
}

func TestValidateTargetURL_LinkLocalRejected(t *testing.T) {
	err := ValidateWebhookURL("http://169.254.1.1/x")
	if err == nil {
		t.Error("expected link-local to be rejected")
	}
}

func TestValidateTargetURL_ValidURLsPass(t *testing.T) {
	valid := []string{
		"https://hooks.slack.com/services/T00/B00/xxx",
		"http://example.com/webhook",
		"https://1.2.3.4:8443/hook",
	}
	for _, u := range valid {
		if err := ValidateWebhookURL(u); err != nil {
			t.Errorf("ValidateWebhookURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestValidateTargetURL_FileSchemeRejected(t *testing.T) {
	rejected := []string{
		"file:///etc/passwd",
		"ftp://example.com/file",
		"gopher://evil.com",
	}
	for _, u := range rejected {
		err := ValidateWebhookURL(u)
		if err == nil {
			t.Errorf("ValidateWebhookURL(%q) = nil, want scheme error", u)
		} else if !strings.Contains(err.Error(), "scheme") {
			t.Errorf("ValidateWebhookURL(%q) = %v, want scheme error", u, err)
		}
	}
}

func TestValidateHealthURL_PrivateIPsAllowed(t *testing.T) {
	allowed := []string{
		"http://10.0.0.1:8080/health",
		"http://192.168.1.1:8080/health",
		"http://172.16.0.1:8080/health",
		"http://127.0.0.1:8080/health",
		"http://localhost:8080/health",
	}
	for _, u := range allowed {
		if err := ValidateHealthURL(u); err != nil {
			t.Errorf("ValidateHealthURL(%q) = %v, want nil (private IPs allowed)", u, err)
		}
	}
}

func TestValidateHealthURL_BadSchemesRejected(t *testing.T) {
	rejected := []string{
		"ftp://10.0.0.1/health",
		"file:///etc/passwd",
		"gopher://x/health",
	}
	for _, u := range rejected {
		err := ValidateHealthURL(u)
		if err == nil {
			t.Errorf("ValidateHealthURL(%q) = nil, want error", u)
		} else if !strings.Contains(err.Error(), "scheme") {
			t.Errorf("ValidateHealthURL(%q) = %v, want scheme error", u, err)
		}
	}
}

func TestValidateHealthURL_EmptyHost(t *testing.T) {
	err := ValidateHealthURL("http:///health")
	if err == nil {
		t.Error("expected empty host error")
	}
}

func TestValidateWebhookURL_EmptyHost(t *testing.T) {
	err := ValidateWebhookURL("http:///hook")
	if err == nil {
		t.Error("expected empty host error")
	}
}
