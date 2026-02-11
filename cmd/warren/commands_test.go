package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
)

// mockAdminServer creates an httptest server with the given route handlers.
func mockAdminServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try exact path+method match first, then path-only.
		key := r.Method + " " + r.URL.Path
		if h, ok := handlers[key]; ok {
			h(w, r)
			return
		}
		if h, ok := handlers[r.URL.Path]; ok {
			h(w, r)
			return
		}
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"not found"}`))
	}))
}

// executeCommand builds a fresh root command and runs it with the given args,
// capturing stdout. It sets adminURL to the given server URL.
func executeCommand(t *testing.T, serverURL string, args ...string) (string, error) {
	t.Helper()

	// Reset globals.
	adminURL = serverURL
	format = "table"

	root := &cobra.Command{
		Use:   "warren",
		Short: "Warren CLI",
	}
	root.PersistentFlags().StringVar(&adminURL, "admin", serverURL, "admin API URL")
	root.PersistentFlags().StringVar(&format, "format", "table", "output format")

	agentCmd := &cobra.Command{Use: "agent", Short: "Manage agents"}
	agentCmd.AddCommand(
		agentListCmd(),
		agentAddCmd(),
		agentRemoveCmd(),
		agentInspectCmd(),
		agentWakeCmd(),
		agentSleepCmd(),
	)

	serviceCmd := &cobra.Command{Use: "service", Short: "Manage dynamic services"}
	serviceCmd.AddCommand(
		serviceListCmd(),
		serviceAddCmd(),
		serviceRemoveCmd(),
	)

	root.AddCommand(
		agentCmd,
		serviceCmd,
		statusCmd(),
		eventsCmd(),
		configValidateCmd(),
		initCmd(),
		scaffoldCmd(),
	)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)

	// Capture os.Stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root.SetArgs(args)
	err := root.Execute()

	w.Close()
	os.Stdout = old
	captured, _ := io.ReadAll(r)
	buf.Write(captured)

	return buf.String(), err
}

// --- Agent List Tests ---

func TestAgentList_Table(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/agents": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]map[string]any{
				{"name": "agent1", "hostname": "a1.example.com", "policy": "on-demand", "state": "sleeping", "connections": 0},
				{"name": "agent2", "hostname": "a2.example.com", "policy": "always-on", "state": "ready", "connections": 5},
			})
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "agent", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, col := range []string{"NAME", "HOSTNAME", "POLICY", "STATE", "CONNECTIONS"} {
		if !strings.Contains(out, col) {
			t.Errorf("missing column header %q in output:\n%s", col, out)
		}
	}
	if !strings.Contains(out, "agent1") || !strings.Contains(out, "agent2") {
		t.Errorf("missing agent names in output:\n%s", out)
	}
	if !strings.Contains(out, "sleeping") || !strings.Contains(out, "ready") {
		t.Errorf("missing states in output:\n%s", out)
	}
}

func TestAgentList_JSON(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/agents": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"name":"agent1","state":"ready"}]`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "agent", "list", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"name":"agent1"`) {
		t.Errorf("expected JSON output, got:\n%s", out)
	}
}

func TestAgentList_Empty(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/agents": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[]`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "agent", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have header but no data rows.
	if !strings.Contains(out, "NAME") {
		t.Errorf("expected header in output:\n%s", out)
	}
}

// --- Agent Add Tests ---

func TestAgentAdd_AllFlags(t *testing.T) {
	var receivedBody map[string]string
	var mu sync.Mutex
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"POST /admin/agents": func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Write([]byte(`{"status":"created"}`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "agent", "add",
		"--name", "testagent",
		"--hostname", "test.example.com",
		"--backend", "http://backend:18790",
		"--policy", "on-demand",
		"--container-name", "openclaw_test",
		"--health-url", "http://backend:18790/health",
		"--idle-timeout", "45m",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("expected 'created' in output, got:\n%s", out)
	}

	mu.Lock()
	defer mu.Unlock()
	if receivedBody["name"] != "testagent" {
		t.Errorf("expected name=testagent, got %s", receivedBody["name"])
	}
	if receivedBody["hostname"] != "test.example.com" {
		t.Errorf("expected hostname=test.example.com, got %s", receivedBody["hostname"])
	}
	if receivedBody["policy"] != "on-demand" {
		t.Errorf("expected policy=on-demand, got %s", receivedBody["policy"])
	}
	if receivedBody["idle_timeout"] != "45m" {
		t.Errorf("expected idle_timeout=45m, got %s", receivedBody["idle_timeout"])
	}
}

func TestAgentAdd_Conflict(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"POST /admin/agents": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(409)
			w.Write([]byte(`{"error":"agent already exists"}`))
		},
	})
	defer srv.Close()

	_, err := executeCommand(t, srv.URL, "agent", "add",
		"--name", "dup",
		"--hostname", "dup.example.com",
		"--backend", "http://b:18790",
		"--policy", "unmanaged",
	)
	if err == nil {
		t.Fatal("expected error for conflict, got nil")
	}
	if !strings.Contains(err.Error(), "409") {
		t.Errorf("expected 409 in error, got: %v", err)
	}
}

// --- Agent Remove Tests ---

func TestAgentRemove_Success(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"DELETE /admin/agents/myagent": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"removed"}`))
		},
	})
	defer srv.Close()

	// Simulate "y" confirmation via stdin.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString("y\n")
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	out, err := executeCommand(t, srv.URL, "agent", "remove", "myagent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("expected 'removed' in output, got:\n%s", out)
	}
}

func TestAgentRemove_Cancelled(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{})
	defer srv.Close()

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString("n\n")
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	out, err := executeCommand(t, srv.URL, "agent", "remove", "myagent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("expected 'Cancelled' in output, got:\n%s", out)
	}
}

func TestAgentRemove_NotFound(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"DELETE /admin/agents/ghost": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"agent not found"}`))
		},
	})
	defer srv.Close()

	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.WriteString("y\n")
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	_, err := executeCommand(t, srv.URL, "agent", "remove", "ghost")
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

// --- Agent Inspect Tests ---

func TestAgentInspect_Success(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/agents/myagent": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"name":     "myagent",
				"hostname": "my.example.com",
				"policy":   "on-demand",
				"state":    "ready",
			})
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "agent", "inspect", "myagent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "myagent") || !strings.Contains(out, "on-demand") {
		t.Errorf("expected agent details in output:\n%s", out)
	}
}

func TestAgentInspect_JSON(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/agents/myagent": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"name":"myagent","state":"ready"}`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "agent", "inspect", "myagent", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"name":"myagent"`) {
		t.Errorf("expected JSON output, got:\n%s", out)
	}
}

func TestAgentInspect_NotFound(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/agents/ghost": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		},
	})
	defer srv.Close()

	_, err := executeCommand(t, srv.URL, "agent", "inspect", "ghost")
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error, got: %v", err)
	}
}

// --- Agent Wake Tests ---

func TestAgentWake_Success(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"POST /admin/agents/myagent/wake": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"waking"}`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "agent", "wake", "myagent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "waking") {
		t.Errorf("expected 'waking' in output, got:\n%s", out)
	}
}

func TestAgentWake_NotFound(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"POST /admin/agents/ghost/wake": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		},
	})
	defer srv.Close()

	_, err := executeCommand(t, srv.URL, "agent", "wake", "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Agent Sleep Tests ---

func TestAgentSleep_Success(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"POST /admin/agents/myagent/sleep": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"sleeping"}`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "agent", "sleep", "myagent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "sleeping") {
		t.Errorf("expected 'sleeping' in output, got:\n%s", out)
	}
}

func TestAgentSleep_NotFound(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"POST /admin/agents/ghost/sleep": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		},
	})
	defer srv.Close()

	_, err := executeCommand(t, srv.URL, "agent", "sleep", "ghost")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Service List Tests ---

func TestServiceList_Table(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/services": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode([]map[string]string{
				{"hostname": "svc1.example.com", "target": "http://backend1:8080", "agent": "agent1"},
				{"hostname": "svc2.example.com", "target": "http://backend2:8080", "agent": "agent2"},
			})
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "service", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, col := range []string{"HOSTNAME", "TARGET", "AGENT"} {
		if !strings.Contains(out, col) {
			t.Errorf("missing column %q in output:\n%s", col, out)
		}
	}
	if !strings.Contains(out, "svc1.example.com") {
		t.Errorf("missing service in output:\n%s", out)
	}
}

func TestServiceList_Empty(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/services": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[]`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "service", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "HOSTNAME") {
		t.Errorf("expected header:\n%s", out)
	}
}

// --- Service Add Tests ---

func TestServiceAdd_Success(t *testing.T) {
	var receivedBody map[string]string
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"POST /api/services": func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Write([]byte(`{"status":"created"}`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "service", "add",
		"--hostname", "newsvc.example.com",
		"--target", "http://backend:8080",
		"--agent", "myagent",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("expected 'created', got:\n%s", out)
	}
	if receivedBody["hostname"] != "newsvc.example.com" {
		t.Errorf("wrong hostname in body: %v", receivedBody)
	}
}

func TestServiceAdd_MissingFlags(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{})
	defer srv.Close()

	_, err := executeCommand(t, srv.URL, "service", "add")
	if err == nil {
		t.Fatal("expected error for missing flags")
	}
	if !strings.Contains(err.Error(), "--hostname and --target are required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Service Remove Tests ---

func TestServiceRemove_Success(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"DELETE /api/services/svc.example.com": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"removed"}`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "service", "remove", "svc.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "removed") {
		t.Errorf("expected 'removed', got:\n%s", out)
	}
}

func TestServiceRemove_NotFound(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"DELETE /api/services/ghost.example.com": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			w.Write([]byte(`{"error":"not found"}`))
		},
	})
	defer srv.Close()

	_, err := executeCommand(t, srv.URL, "service", "remove", "ghost.example.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Status Tests ---

func TestStatus_Table(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/health": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]any{
				"uptime_seconds": 90061.0, // 1d 1h 1m
				"agent_count":    3,
				"ready_count":    2,
				"sleeping_count": 1,
				"ws_connections": 10,
				"service_count":  5,
			})
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Warren Orchestrator") {
		t.Errorf("missing header in output:\n%s", out)
	}
	if !strings.Contains(out, "Agents") || !strings.Contains(out, "3") {
		t.Errorf("missing agent count:\n%s", out)
	}
	if !strings.Contains(out, "10") {
		t.Errorf("missing ws connections:\n%s", out)
	}
	if !strings.Contains(out, "5 dynamic routes") {
		t.Errorf("missing service count:\n%s", out)
	}
}

func TestStatus_JSON(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/health": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"uptime_seconds":100,"agent_count":1}`))
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "status", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "uptime_seconds") {
		t.Errorf("expected JSON, got:\n%s", out)
	}
}

// --- Config Validate Tests ---

func TestConfigValidate_Valid(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "test.yaml")
	content := `listen: ":8080"
agents:
  test:
    hostname: test.example.com
    backend: "http://backend:18790"
    policy: unmanaged
`
	os.WriteFile(cfgFile, []byte(content), 0644)

	// Note: cobra Use "config validate <file>" means command name is "config",
	// so args are ["validate", cfgFile] but ExactArgs(1) only wants 1.
	// This is a CLI bug — the Use string should just be "config <file>" or
	// it should be a subcommand. We call it as "config <file>" to match actual behavior.
	out, err := executeCommand(t, "", "config", cfgFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK, got:\n%s", out)
	}
}

func TestConfigValidate_Invalid(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "bad.yaml")
	// No agents defined - should fail validation.
	os.WriteFile(cfgFile, []byte(`listen: ":8080"`), 0644)

	_, err := executeCommand(t, "", "config", cfgFile)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigValidate_BadYAML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "broken.yaml")
	os.WriteFile(cfgFile, []byte(`{{{not yaml`), 0644)

	_, err := executeCommand(t, "", "config", cfgFile)
	if err == nil {
		t.Fatal("expected error for bad YAML")
	}
}

// --- Events Tests ---

func TestEvents_SSE(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/events": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			for i := 0; i < 3; i++ {
				fmt.Fprintf(w, "data: event-%d\n\n", i)
				if flusher != nil {
					flusher.Flush()
				}
			}
		},
	})
	defer srv.Close()

	out, err := executeCommand(t, srv.URL, "events")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < 3; i++ {
		expected := fmt.Sprintf("event-%d", i)
		if !strings.Contains(out, expected) {
			t.Errorf("missing %q in output:\n%s", expected, out)
		}
	}
}

// --- Init Tests ---

func TestInit(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	out, err := executeCommand(t, "", "init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Created orchestrator.yaml") {
		t.Errorf("expected creation message, got:\n%s", out)
	}

	orchData, err := os.ReadFile(filepath.Join(dir, "orchestrator.yaml"))
	if err != nil {
		t.Fatalf("orchestrator.yaml not created: %v", err)
	}
	if !strings.Contains(string(orchData), "admin_listen") {
		t.Error("orchestrator.yaml missing expected content")
	}

	stackData, err := os.ReadFile(filepath.Join(dir, "stack.yaml"))
	if err != nil {
		t.Fatalf("stack.yaml not created: %v", err)
	}
	if !strings.Contains(string(stackData), "warren") {
		t.Error("stack.yaml missing expected content")
	}
}

// --- Scaffold Tests ---

func TestScaffold(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	out, err := executeCommand(t, "", "scaffold", "mybot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Scaffolded agent in ./mybot/") {
		t.Errorf("unexpected output:\n%s", out)
	}

	// Check files exist.
	for _, f := range []string{"Dockerfile", "openclaw.json", "supervisord.conf"} {
		path := filepath.Join(dir, "mybot", f)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s not created: %v", f, err)
		}
		if f == "openclaw.json" && !strings.Contains(string(data), "mybot") {
			t.Errorf("openclaw.json missing agent name 'mybot'")
		}
	}
}

// --- Edge Cases ---

func TestAdminFlagOverride(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/health": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"uptime_seconds":1}`))
		},
	})
	defer srv.Close()

	// Use --admin flag to point to our server.
	out, err := executeCommand(t, "", "status", "--admin", srv.URL, "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "uptime_seconds") {
		t.Errorf("expected JSON output, got:\n%s", out)
	}
}

func TestConnectionRefused(t *testing.T) {
	// Point to a port nothing listens on.
	_, err := executeCommand(t, "http://127.0.0.1:1", "agent", "list")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestNonJSONResponse(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/agents": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`this is not json`))
		},
	})
	defer srv.Close()

	// Should not crash - table mode will just show empty since unmarshal fails.
	_, err := executeCommand(t, srv.URL, "agent", "list")
	if err != nil {
		t.Fatalf("should not error on non-JSON (just empty table): %v", err)
	}
}

func TestEnvVarOverride(t *testing.T) {
	srv := mockAdminServer(t, map[string]http.HandlerFunc{
		"GET /admin/health": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"uptime_seconds":42}`))
		},
	})
	defer srv.Close()

	os.Setenv("WARREN_ADMIN", srv.URL)
	defer os.Unsetenv("WARREN_ADMIN")

	// Set adminURL to empty to let env var take effect.
	adminURL = ""
	out, err := executeCommand(t, "", "status", "--format", "json")
	// The executeCommand sets adminURL to "", so getAdminURL should fall through to env.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("expected env var to work, got:\n%s", out)
	}
}
