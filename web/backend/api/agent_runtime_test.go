package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	ppid "github.com/sipeed/picoclaw/pkg/pid"
)

func TestHandleAgentRuntime_ProxySuccess(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.DefaultConfig()
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	originalRead := readGatewayPIDDataForRuntime
	originalDo := gatewayRuntimeDo
	t.Cleanup(func() {
		readGatewayPIDDataForRuntime = originalRead
		gatewayRuntimeDo = originalDo
	})

	readGatewayPIDDataForRuntime = func() *ppid.PidFileData {
		return &ppid.PidFileData{PID: 123, Host: "127.0.0.1", Port: 18790, Token: "pid-secret"}
	}
	gatewayRuntimeDo = func(req *http.Request, timeout time.Duration) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "Bearer pid-secret" {
			t.Fatalf("Authorization = %q, want Bearer pid-secret", got)
		}
		if req.URL.String() != "http://127.0.0.1:18790/runtime/agent" {
			t.Fatalf("URL = %q", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"generated_at": 1,
				"summary": {"agent_count": 1, "teammate_count": 1, "task_count": 1, "task_statuses": {"queued": 1}},
				"agents": [{"id":"main","model":"gpt-4o"}],
				"teammates": [{"id":"reviewer","agent_id":"main","memory_scope":"teammate:reviewer"}],
				"tasks": [{"owner_agent_id":"main","id":"subagent-1","status":"queued"}]
			}`)),
			Header: make(http.Header),
		}, nil
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.registerAgentRuntimeRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/agent/runtime", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"teammates"`) {
		t.Fatalf("response missing teammates: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"tasks"`) {
		t.Fatalf("response missing tasks: %s", rec.Body.String())
	}
}

func TestHandleAgentRuntime_GatewayNotRunning(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.DefaultConfig()
	if err := config.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("SaveConfig() error = %v", err)
	}

	originalRead := readGatewayPIDDataForRuntime
	t.Cleanup(func() {
		readGatewayPIDDataForRuntime = originalRead
	})
	readGatewayPIDDataForRuntime = func() *ppid.PidFileData { return nil }

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.registerAgentRuntimeRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/agent/runtime", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}
