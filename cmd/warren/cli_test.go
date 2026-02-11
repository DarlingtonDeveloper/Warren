package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGetAdminURLDefault(t *testing.T) {
	// Clear env.
	os.Unsetenv("WARREN_ADMIN")
	adminURL = ""

	result := getAdminURL()
	if result != "http://localhost:9090" {
		t.Fatalf("expected default URL, got %s", result)
	}
}

func TestGetAdminURLEnv(t *testing.T) {
	os.Setenv("WARREN_ADMIN", "http://custom:1234")
	defer os.Unsetenv("WARREN_ADMIN")
	adminURL = ""

	result := getAdminURL()
	if result != "http://custom:1234" {
		t.Fatalf("expected env URL, got %s", result)
	}
}

func TestGetAdminURLFlag(t *testing.T) {
	os.Setenv("WARREN_ADMIN", "http://env:1234")
	defer os.Unsetenv("WARREN_ADMIN")
	adminURL = "http://flag:5678"

	result := getAdminURL()
	if result != "http://flag:5678" {
		t.Fatalf("expected flag URL, got %s", result)
	}
	adminURL = ""
}

func TestGetAdminURLConfigFile(t *testing.T) {
	os.Unsetenv("WARREN_ADMIN")
	adminURL = ""

	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".warren")
	os.MkdirAll(dir, 0755)
	cfgPath := filepath.Join(dir, "config.yaml")

	// Write test config.
	data, _ := yaml.Marshal(map[string]string{"admin": "http://fromfile:9999"})
	os.WriteFile(cfgPath, data, 0644)
	defer os.Remove(cfgPath)

	result := getAdminURL()
	if result != "http://fromfile:9999" {
		t.Fatalf("expected config file URL, got %s", result)
	}
}
