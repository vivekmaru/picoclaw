package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/config"
	ppid "github.com/sipeed/picoclaw/pkg/pid"
)

const gatewayRuntimeRequestTimeout = 3 * time.Second

var (
	readGatewayPIDDataForRuntime = func() *ppid.PidFileData {
		return ppid.ReadPidFileWithCheck(globalConfigDir())
	}
	gatewayRuntimeDo = func(req *http.Request, timeout time.Duration) (*http.Response, error) {
		client := http.Client{Timeout: timeout}
		return client.Do(req)
	}
)

func (h *Handler) registerAgentRuntimeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agent/runtime", h.handleAgentRuntime)
	mux.HandleFunc("GET /api/agent/runtime/tasks/{ownerAgentID}/{taskID}", h.handleAgentRuntimeTask)
	mux.HandleFunc("POST /api/agent/runtime/tasks/{ownerAgentID}/{taskID}/cancel", h.handleCancelAgentRuntimeTask)
	mux.HandleFunc("POST /api/agent/runtime/tasks/{ownerAgentID}/{taskID}/approve", h.handleApproveAgentRuntimeTask)
	mux.HandleFunc("POST /api/agent/runtime/tasks/{ownerAgentID}/{taskID}/reject", h.handleRejectAgentRuntimeTask)
	mux.HandleFunc("POST /api/agent/runtime/tasks/{ownerAgentID}/{taskID}/memory-proposals", h.handleCreateAgentRuntimeMemoryProposal)
	mux.HandleFunc("POST /api/agent/runtime/memory-proposals/{ownerAgentID}/{proposalID}/approve", h.handleApproveAgentRuntimeMemoryProposal)
	mux.HandleFunc("POST /api/agent/runtime/memory-proposals/{ownerAgentID}/{proposalID}/reject", h.handleRejectAgentRuntimeMemoryProposal)
}

func (h *Handler) handleAgentRuntime(w http.ResponseWriter, r *http.Request) {
	snapshot, statusCode, err := h.getGatewayRuntimeSnapshot(gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleAgentRuntimeTask(w http.ResponseWriter, r *http.Request) {
	ownerAgentID := r.PathValue("ownerAgentID")
	taskID := r.PathValue("taskID")

	task, statusCode, err := h.getGatewayRuntimeTask(ownerAgentID, taskID, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleCancelAgentRuntimeTask(w http.ResponseWriter, r *http.Request) {
	ownerAgentID := r.PathValue("ownerAgentID")
	taskID := r.PathValue("taskID")

	task, statusCode, err := h.cancelGatewayRuntimeTask(ownerAgentID, taskID, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleApproveAgentRuntimeTask(w http.ResponseWriter, r *http.Request) {
	ownerAgentID := r.PathValue("ownerAgentID")
	taskID := r.PathValue("taskID")

	task, statusCode, err := h.approveGatewayRuntimeTask(ownerAgentID, taskID, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleRejectAgentRuntimeTask(w http.ResponseWriter, r *http.Request) {
	ownerAgentID := r.PathValue("ownerAgentID")
	taskID := r.PathValue("taskID")

	task, statusCode, err := h.rejectGatewayRuntimeTask(ownerAgentID, taskID, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleCreateAgentRuntimeMemoryProposal(w http.ResponseWriter, r *http.Request) {
	ownerAgentID := r.PathValue("ownerAgentID")
	taskID := r.PathValue("taskID")
	var req struct {
		Scope string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	proposal, statusCode, err := h.createGatewayRuntimeMemoryProposal(ownerAgentID, taskID, req.Scope, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proposal); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleApproveAgentRuntimeMemoryProposal(w http.ResponseWriter, r *http.Request) {
	ownerAgentID := r.PathValue("ownerAgentID")
	proposalID := r.PathValue("proposalID")

	proposal, statusCode, err := h.approveGatewayRuntimeMemoryProposal(ownerAgentID, proposalID, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proposal); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleRejectAgentRuntimeMemoryProposal(w http.ResponseWriter, r *http.Request) {
	ownerAgentID := r.PathValue("ownerAgentID")
	proposalID := r.PathValue("proposalID")

	proposal, statusCode, err := h.rejectGatewayRuntimeMemoryProposal(ownerAgentID, proposalID, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proposal); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) getGatewayRuntimeSnapshot(timeout time.Duration) (*agent.RuntimeSnapshot, int, error) {
	resp, statusCode, err := h.doGatewayRuntimeRequest(http.MethodGet, "/runtime/agent", nil, timeout)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var snapshot agent.RuntimeSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &snapshot, http.StatusOK, nil
}

func (h *Handler) getGatewayRuntimeTask(ownerAgentID, taskID string, timeout time.Duration) (*agent.RuntimeTaskInfo, int, error) {
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodGet,
		"/runtime/agent/tasks/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(taskID),
		nil,
		timeout,
	)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var task agent.RuntimeTaskInfo
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &task, http.StatusOK, nil
}

func (h *Handler) cancelGatewayRuntimeTask(ownerAgentID, taskID string, timeout time.Duration) (*agent.RuntimeTaskInfo, int, error) {
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/tasks/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(taskID)+"/cancel",
		nil,
		timeout,
	)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var task agent.RuntimeTaskInfo
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &task, http.StatusOK, nil
}

func (h *Handler) approveGatewayRuntimeTask(ownerAgentID, taskID string, timeout time.Duration) (*agent.RuntimeTaskInfo, int, error) {
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/tasks/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(taskID)+"/approve",
		nil,
		timeout,
	)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var task agent.RuntimeTaskInfo
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &task, http.StatusOK, nil
}

func (h *Handler) rejectGatewayRuntimeTask(ownerAgentID, taskID string, timeout time.Duration) (*agent.RuntimeTaskInfo, int, error) {
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/tasks/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(taskID)+"/reject",
		nil,
		timeout,
	)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var task agent.RuntimeTaskInfo
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &task, http.StatusOK, nil
}

func (h *Handler) createGatewayRuntimeMemoryProposal(ownerAgentID, taskID, scope string, timeout time.Duration) (*agent.RuntimeMemoryProposalInfo, int, error) {
	var body io.Reader
	if strings.TrimSpace(scope) != "" {
		payload, err := json.Marshal(map[string]string{"scope": scope})
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		body = bytes.NewReader(payload)
	}
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/tasks/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(taskID)+"/memory-proposals",
		body,
		timeout,
	)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var proposal agent.RuntimeMemoryProposalInfo
	if err := json.NewDecoder(resp.Body).Decode(&proposal); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &proposal, http.StatusOK, nil
}

func (h *Handler) approveGatewayRuntimeMemoryProposal(ownerAgentID, proposalID string, timeout time.Duration) (*agent.RuntimeMemoryProposalInfo, int, error) {
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/memory-proposals/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(proposalID)+"/approve",
		nil,
		timeout,
	)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var proposal agent.RuntimeMemoryProposalInfo
	if err := json.NewDecoder(resp.Body).Decode(&proposal); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &proposal, http.StatusOK, nil
}

func (h *Handler) rejectGatewayRuntimeMemoryProposal(ownerAgentID, proposalID string, timeout time.Duration) (*agent.RuntimeMemoryProposalInfo, int, error) {
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/memory-proposals/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(proposalID)+"/reject",
		nil,
		timeout,
	)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var proposal agent.RuntimeMemoryProposalInfo
	if err := json.NewDecoder(resp.Body).Decode(&proposal); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &proposal, http.StatusOK, nil
}

func (h *Handler) doGatewayRuntimeRequest(
	method, path string,
	body io.Reader,
	timeout time.Duration,
) (*http.Response, int, error) {
	cfg, _ := config.LoadConfig(h.configPath)
	pidData := readGatewayPIDDataForRuntime()
	if pidData == nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("gateway is not running")
	}

	host := pidData.Host
	port := pidData.Port
	if port <= 0 {
		port = 18790
		if cfg != nil && cfg.Gateway.Port != 0 {
			port = cfg.Gateway.Port
		}
	}
	if host == "" {
		host = gatewayProbeHost(h.effectiveGatewayBindHost(cfg))
	}

	req, err := http.NewRequest(method, "http://"+net.JoinHostPort(host, strconv.Itoa(port))+path, body)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if pidData.Token != "" {
		req.Header.Set("Authorization", "Bearer "+pidData.Token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := gatewayRuntimeDo(req, timeout)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		msg := fmt.Sprintf("gateway runtime endpoint returned %s", resp.Status)
		if data, readErr := io.ReadAll(resp.Body); readErr == nil && len(data) > 0 {
			msg = strings.TrimSpace(string(data))
		}
		return nil, resp.StatusCode, fmt.Errorf("%s", msg)
	}
	return resp, http.StatusOK, nil
}
