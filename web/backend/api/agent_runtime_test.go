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

func TestHandleAgentRuntimeTask_ProxySuccess(t *testing.T) {
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
		if req.Method != http.MethodGet {
			t.Fatalf("Method = %s, want GET", req.Method)
		}
		if req.URL.String() != "http://127.0.0.1:18790/runtime/agent/tasks/main/subagent-9" {
			t.Fatalf("URL = %q", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"owner_agent_id":"main",
				"id":"subagent-9",
				"status":"running",
				"cancelable":true
			}`)),
			Header: make(http.Header),
		}, nil
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.registerAgentRuntimeRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/agent/runtime/tasks/main/subagent-9", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"cancelable":true`) {
		t.Fatalf("response missing cancelable flag: %s", rec.Body.String())
	}
}

func TestHandleCancelAgentRuntimeTask_ProxyConflict(t *testing.T) {
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
		if req.Method != http.MethodPost {
			t.Fatalf("Method = %s, want POST", req.Method)
		}
		if req.URL.String() != "http://127.0.0.1:18790/runtime/agent/tasks/main/subagent-9/cancel" {
			t.Fatalf("URL = %q", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusConflict,
			Status:     "409 Conflict",
			Body:       io.NopCloser(strings.NewReader(`task "subagent-9" is not cancelable`)),
			Header:     make(http.Header),
		}, nil
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.registerAgentRuntimeRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/agent/runtime/tasks/main/subagent-9/cancel", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not cancelable") {
		t.Fatalf("body = %q, want cancelable error", rec.Body.String())
	}
}

func TestHandleApproveAgentRuntimeTask_ProxySuccess(t *testing.T) {
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
		if req.URL.String() != "http://127.0.0.1:18790/runtime/agent/tasks/main/subagent-1/approve" {
			t.Fatalf("URL = %q", req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"actor":"operator"`) || !strings.Contains(string(body), `"note":"Looks safe"`) {
			t.Fatalf("request body = %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"owner_agent_id":"main",
				"id":"subagent-1",
				"status":"queued"
			}`)),
			Header: make(http.Header),
		}, nil
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.registerAgentRuntimeRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/agent/runtime/tasks/main/subagent-1/approve", strings.NewReader(`{"actor":"operator","note":"Looks safe"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"queued"`) {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func TestHandleCreateAgentRuntimeMemoryProposal_ProxySuccess(t *testing.T) {
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
		if req.Method != http.MethodPost {
			t.Fatalf("Method = %s, want POST", req.Method)
		}
		if req.URL.String() != "http://127.0.0.1:18790/runtime/agent/tasks/main/subagent-1/memory-proposals" {
			t.Fatalf("URL = %q", req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"scope":"shared"`) {
			t.Fatalf("request body = %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"owner_agent_id":"main",
				"id":"memory-1",
				"scope":"shared",
				"target":"long_term",
				"kind":"task_result",
				"status":"pending",
				"content":"remember this"
			}`)),
			Header: make(http.Header),
		}, nil
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.registerAgentRuntimeRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/agent/runtime/tasks/main/subagent-1/memory-proposals", strings.NewReader(`{"scope":"shared"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"memory-1"`) {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func TestHandleApproveAgentRuntimeMemoryProposal_ProxySuccess(t *testing.T) {
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
		if req.URL.String() != "http://127.0.0.1:18790/runtime/agent/memory-proposals/main/memory-1/approve" {
			t.Fatalf("URL = %q", req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		if !strings.Contains(string(body), `"actor":"operator"`) || !strings.Contains(string(body), `"note":"Keep this"`) {
			t.Fatalf("request body = %s", string(body))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"owner_agent_id":"main",
				"id":"memory-1",
				"scope":"shared",
				"target":"long_term",
				"kind":"task_result",
				"status":"approved",
				"content":"remember this"
			}`)),
			Header: make(http.Header),
		}, nil
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.registerAgentRuntimeRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/agent/runtime/memory-proposals/main/memory-1/approve", strings.NewReader(`{"actor":"operator","note":"Keep this"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"approved"`) {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func TestHandleUpdateAgentRuntimeMemoryProposal_ProxySuccess(t *testing.T) {
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
		if req.URL.String() != "http://127.0.0.1:18790/runtime/agent/memory-proposals/main/memory-1/update" {
			t.Fatalf("URL = %q", req.URL.String())
		}
		body, _ := io.ReadAll(req.Body)
		payload := string(body)
		if !strings.Contains(payload, `"actor":"operator"`) ||
			!strings.Contains(payload, `"scope":"teammate:reviewer"`) ||
			!strings.Contains(payload, `"title":"Edited"`) ||
			!strings.Contains(payload, `"content":"Remember this instead"`) {
			t.Fatalf("request body = %s", payload)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"owner_agent_id":"main",
				"id":"memory-1",
				"scope":"teammate:reviewer",
				"target":"long_term",
				"kind":"task_result",
				"status":"pending",
				"title":"Edited",
				"content":"Remember this instead",
				"updated_by":"operator"
			}`)),
			Header: make(http.Header),
		}, nil
	}

	h := NewHandler(configPath)
	mux := http.NewServeMux()
	h.registerAgentRuntimeRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/agent/runtime/memory-proposals/main/memory-1/update", strings.NewReader(`{"actor":"operator","scope":"teammate:reviewer","title":"Edited","content":"Remember this instead"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"updated_by":"operator"`) {
		t.Fatalf("response = %s", rec.Body.String())
	}
}
