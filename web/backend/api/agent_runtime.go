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
	mux.HandleFunc("GET /api/agent/runtime/memory-catalog", h.handleAgentRuntimeMemoryCatalog)
	mux.HandleFunc("GET /api/agent/runtime/memory-catalog/export", h.handleExportAgentRuntimeMemoryCatalog)
	mux.HandleFunc("POST /api/agent/runtime/memory-catalog/entries/{entryID}/pin", h.handlePinAgentRuntimeMemoryCatalogEntry)
	mux.HandleFunc("POST /api/agent/runtime/memory-catalog/entries/{entryID}/unpin", h.handleUnpinAgentRuntimeMemoryCatalogEntry)
	mux.HandleFunc("POST /api/agent/runtime/memory-catalog/entries/{entryID}/archive", h.handleArchiveAgentRuntimeMemoryCatalogEntry)
	mux.HandleFunc("POST /api/agent/runtime/memory-catalog/entries/{entryID}/restore", h.handleRestoreAgentRuntimeMemoryCatalogEntry)
	mux.HandleFunc("GET /api/agent/runtime/tasks/{ownerAgentID}/{taskID}", h.handleAgentRuntimeTask)
	mux.HandleFunc("POST /api/agent/runtime/tasks/{ownerAgentID}/{taskID}/cancel", h.handleCancelAgentRuntimeTask)
	mux.HandleFunc("POST /api/agent/runtime/tasks/{ownerAgentID}/{taskID}/approve", h.handleApproveAgentRuntimeTask)
	mux.HandleFunc("POST /api/agent/runtime/tasks/{ownerAgentID}/{taskID}/reject", h.handleRejectAgentRuntimeTask)
	mux.HandleFunc("POST /api/agent/runtime/tasks/{ownerAgentID}/{taskID}/handoff", h.handleHandoffAgentRuntimeTask)
	mux.HandleFunc("POST /api/agent/runtime/tasks/{ownerAgentID}/{taskID}/memory-proposals", h.handleCreateAgentRuntimeMemoryProposal)
	mux.HandleFunc("POST /api/agent/runtime/memory-proposals/{ownerAgentID}/{proposalID}/update", h.handleUpdateAgentRuntimeMemoryProposal)
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

func (h *Handler) handleAgentRuntimeMemoryCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, statusCode, err := h.getGatewayRuntimeMemoryCatalog(gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(catalog); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleExportAgentRuntimeMemoryCatalog(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	payload, contentType, filename, statusCode, err := h.exportGatewayRuntimeMemoryCatalog(format, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = w.Write(payload)
}

func (h *Handler) handlePinAgentRuntimeMemoryCatalogEntry(w http.ResponseWriter, r *http.Request) {
	h.handleAgentRuntimeMemoryCatalogEntryAction(w, r, "pin")
}

func (h *Handler) handleUnpinAgentRuntimeMemoryCatalogEntry(w http.ResponseWriter, r *http.Request) {
	h.handleAgentRuntimeMemoryCatalogEntryAction(w, r, "unpin")
}

func (h *Handler) handleArchiveAgentRuntimeMemoryCatalogEntry(w http.ResponseWriter, r *http.Request) {
	h.handleAgentRuntimeMemoryCatalogEntryAction(w, r, "archive")
}

func (h *Handler) handleRestoreAgentRuntimeMemoryCatalogEntry(w http.ResponseWriter, r *http.Request) {
	h.handleAgentRuntimeMemoryCatalogEntryAction(w, r, "restore")
}

func (h *Handler) handleAgentRuntimeMemoryCatalogEntryAction(w http.ResponseWriter, r *http.Request, action string) {
	entryID := r.PathValue("entryID")
	var req struct {
		Actor string `json:"actor"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	entry, statusCode, err := h.updateGatewayRuntimeMemoryCatalogEntry(entryID, action, req.Actor, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entry); err != nil {
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
	var req struct {
		Actor string `json:"actor"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	task, statusCode, err := h.approveGatewayRuntimeTask(ownerAgentID, taskID, req.Actor, req.Note, gatewayRuntimeRequestTimeout)
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
	var req struct {
		Actor string `json:"actor"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	task, statusCode, err := h.rejectGatewayRuntimeTask(ownerAgentID, taskID, req.Actor, req.Note, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(task); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleHandoffAgentRuntimeTask(w http.ResponseWriter, r *http.Request) {
	ownerAgentID := r.PathValue("ownerAgentID")
	taskID := r.PathValue("taskID")
	var req agent.RuntimeTaskHandoffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	task, statusCode, err := h.handoffGatewayRuntimeTask(ownerAgentID, taskID, req, gatewayRuntimeRequestTimeout)
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
	var req struct {
		Actor string `json:"actor"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	proposal, statusCode, err := h.approveGatewayRuntimeMemoryProposal(ownerAgentID, proposalID, req.Actor, req.Note, gatewayRuntimeRequestTimeout)
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
	var req struct {
		Actor string `json:"actor"`
		Note  string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	proposal, statusCode, err := h.rejectGatewayRuntimeMemoryProposal(ownerAgentID, proposalID, req.Actor, req.Note, gatewayRuntimeRequestTimeout)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(proposal); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *Handler) handleUpdateAgentRuntimeMemoryProposal(w http.ResponseWriter, r *http.Request) {
	ownerAgentID := r.PathValue("ownerAgentID")
	proposalID := r.PathValue("proposalID")
	var req struct {
		Actor      string `json:"actor"`
		Scope      string `json:"scope"`
		Domain     string `json:"domain"`
		EntryType  string `json:"entry_type"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		Confidence string `json:"confidence"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	proposal, statusCode, err := h.updateGatewayRuntimeMemoryProposal(
		ownerAgentID,
		proposalID,
		req.Actor,
		req.Scope,
		req.Domain,
		req.EntryType,
		req.Title,
		req.Content,
		req.Confidence,
		gatewayRuntimeRequestTimeout,
	)
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

func (h *Handler) getGatewayRuntimeMemoryCatalog(timeout time.Duration) (*agent.RuntimeMemoryCatalog, int, error) {
	resp, statusCode, err := h.doGatewayRuntimeRequest(http.MethodGet, "/runtime/agent/memory-catalog", nil, timeout)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var catalog agent.RuntimeMemoryCatalog
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &catalog, http.StatusOK, nil
}

func (h *Handler) updateGatewayRuntimeMemoryCatalogEntry(entryID, action, actor string, timeout time.Duration) (*agent.RuntimeMemoryEntryInfo, int, error) {
	var body io.Reader
	if strings.TrimSpace(actor) != "" {
		payload, err := json.Marshal(map[string]string{"actor": actor})
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		body = bytes.NewReader(payload)
	}
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/memory-catalog/entries/"+url.PathEscape(entryID)+"/"+url.PathEscape(action),
		body,
		timeout,
	)
	if err != nil {
		return nil, statusCode, err
	}
	defer resp.Body.Close()

	var entry agent.RuntimeMemoryEntryInfo
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &entry, http.StatusOK, nil
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

func (h *Handler) approveGatewayRuntimeTask(ownerAgentID, taskID, actor, note string, timeout time.Duration) (*agent.RuntimeTaskInfo, int, error) {
	var body io.Reader
	if strings.TrimSpace(actor) != "" || strings.TrimSpace(note) != "" {
		payload, err := json.Marshal(map[string]string{"actor": actor, "note": note})
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		body = bytes.NewReader(payload)
	}
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/tasks/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(taskID)+"/approve",
		body,
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

func (h *Handler) rejectGatewayRuntimeTask(ownerAgentID, taskID, actor, note string, timeout time.Duration) (*agent.RuntimeTaskInfo, int, error) {
	var body io.Reader
	if strings.TrimSpace(actor) != "" || strings.TrimSpace(note) != "" {
		payload, err := json.Marshal(map[string]string{"actor": actor, "note": note})
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		body = bytes.NewReader(payload)
	}
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/tasks/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(taskID)+"/reject",
		body,
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

func (h *Handler) handoffGatewayRuntimeTask(ownerAgentID, taskID string, req agent.RuntimeTaskHandoffRequest, timeout time.Duration) (*agent.RuntimeTaskInfo, int, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/tasks/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(taskID)+"/handoff",
		bytes.NewReader(payload),
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

func (h *Handler) updateGatewayRuntimeMemoryProposal(ownerAgentID, proposalID, actor, scope, domain, entryType, title, content, confidence string, timeout time.Duration) (*agent.RuntimeMemoryProposalInfo, int, error) {
	payload, err := json.Marshal(map[string]string{
		"actor":      actor,
		"scope":      scope,
		"domain":     domain,
		"entry_type": entryType,
		"title":      title,
		"content":    content,
		"confidence": confidence,
	})
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/memory-proposals/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(proposalID)+"/update",
		bytes.NewReader(payload),
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

func (h *Handler) approveGatewayRuntimeMemoryProposal(ownerAgentID, proposalID, actor, note string, timeout time.Duration) (*agent.RuntimeMemoryProposalInfo, int, error) {
	var body io.Reader
	if strings.TrimSpace(actor) != "" || strings.TrimSpace(note) != "" {
		payload, err := json.Marshal(map[string]string{"actor": actor, "note": note})
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		body = bytes.NewReader(payload)
	}
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/memory-proposals/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(proposalID)+"/approve",
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

func (h *Handler) rejectGatewayRuntimeMemoryProposal(ownerAgentID, proposalID, actor, note string, timeout time.Duration) (*agent.RuntimeMemoryProposalInfo, int, error) {
	var body io.Reader
	if strings.TrimSpace(actor) != "" || strings.TrimSpace(note) != "" {
		payload, err := json.Marshal(map[string]string{"actor": actor, "note": note})
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		body = bytes.NewReader(payload)
	}
	resp, statusCode, err := h.doGatewayRuntimeRequest(
		http.MethodPost,
		"/runtime/agent/memory-proposals/"+url.PathEscape(ownerAgentID)+"/"+url.PathEscape(proposalID)+"/reject",
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

func (h *Handler) exportGatewayRuntimeMemoryCatalog(format string, timeout time.Duration) ([]byte, string, string, int, error) {
	path := "/runtime/agent/memory-catalog/export"
	if strings.TrimSpace(format) != "" {
		path += "?format=" + url.QueryEscape(format)
	}
	resp, statusCode, err := h.doGatewayRuntimeRequest(http.MethodGet, path, nil, timeout)
	if err != nil {
		return nil, "", "", statusCode, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", http.StatusBadGateway, err
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := parseDownloadFilename(resp.Header.Get("Content-Disposition"))
	if filename == "" {
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "json":
			filename = "memory-catalog.json"
		default:
			filename = "memory-catalog.md"
		}
	}
	return payload, contentType, filename, http.StatusOK, nil
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

func parseDownloadFilename(contentDisposition string) string {
	contentDisposition = strings.TrimSpace(contentDisposition)
	if contentDisposition == "" {
		return ""
	}
	for _, part := range strings.Split(contentDisposition, ";") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(strings.ToLower(part), "filename=") {
			continue
		}
		filename := strings.TrimSpace(strings.TrimPrefix(part, "filename="))
		return strings.Trim(filename, `"`)
	}
	return ""
}
