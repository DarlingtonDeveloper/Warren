package security

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// ValidateHostname validates a hostname against RFC 1123.
func ValidateHostname(hostname string) error {
	if hostname == "" {
		return fmt.Errorf("hostname is empty")
	}
	if len(hostname) > 253 {
		return fmt.Errorf("hostname exceeds 253 characters")
	}

	labelRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

	labels := strings.Split(hostname, ".")
	for _, label := range labels {
		if len(label) == 0 {
			return fmt.Errorf("hostname contains empty label")
		}
		if len(label) > 63 {
			return fmt.Errorf("hostname label %q exceeds 63 characters", label)
		}
		if !labelRegex.MatchString(label) {
			return fmt.Errorf("hostname label %q contains invalid characters", label)
		}
	}
	return nil
}

// ValidateWebhookURL validates a webhook URL, rejecting private/internal IPs (SSRF protection).
func ValidateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme %q not allowed, must be http or https", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}

	// Resolve hostname to check for private IPs.
	if ip := net.ParseIP(host); ip != nil {
		if err := rejectPrivateIP(ip); err != nil {
			return err
		}
	} else {
		// It's a hostname — resolve it.
		ips, err := net.LookupIP(host)
		if err != nil {
			// Can't resolve at config time — allow but it may fail at runtime.
			return nil
		}
		for _, ip := range ips {
			if err := rejectPrivateIP(ip); err != nil {
				return fmt.Errorf("host %q resolves to %s: %w", host, ip, err)
			}
		}
	}
	return nil
}

// ValidateHealthURL validates a health check URL (allows private IPs since health checks target containers).
func ValidateHealthURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("malformed URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme %q not allowed, must be http or https", u.Scheme)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("empty host")
	}
	return nil
}

func rejectPrivateIP(ip net.IP) error {
	if ip.IsLoopback() {
		return fmt.Errorf("loopback address %s not allowed", ip)
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("link-local address %s not allowed", ip)
	}

	// Check RFC 1918 private ranges.
	privateRanges := []struct {
		network string
		label   string
	}{
		{"10.0.0.0/8", "10.x.x.x"},
		{"172.16.0.0/12", "172.16-31.x.x"},
		{"192.168.0.0/16", "192.168.x.x"},
		{"169.254.0.0/16", "link-local"},
	}
	for _, pr := range privateRanges {
		_, cidr, _ := net.ParseCIDR(pr.network)
		if cidr.Contains(ip) {
			return fmt.Errorf("private IP %s (%s) not allowed", ip, pr.label)
		}
	}
	return nil
}
