package gateway

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/agent"
)

const gatewayRuntimePath = "/runtime/agent"
const gatewayRuntimeTasksPrefix = "/runtime/agent/tasks/"
const gatewayRuntimeMemoryProposalsPrefix = "/runtime/agent/memory-proposals/"

type runtimeTaskMemoryProposalRequest struct {
	Scope string `json:"scope"`
}

type runtimeReviewActionRequest struct {
	Actor string `json:"actor"`
	Note  string `json:"note"`
}

type runtimeMemoryProposalUpdateRequest struct {
	Actor   string `json:"actor"`
	Scope   string `json:"scope"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

func registerRuntimeHTTPHandlers(agentLoop *agent.AgentLoop, channelManager httpHandlerRegistrar, authToken string) {
	if agentLoop == nil || channelManager == nil {
		return
	}
	channelManager.RegisterHTTPHandlerFunc(gatewayRuntimePath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
			return
		}
		if !authorizedGatewayRuntimeRequest(r, authToken) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentLoop.GetRuntimeSnapshot())
	})

	channelManager.RegisterHTTPHandlerFunc(gatewayRuntimeTasksPrefix, func(w http.ResponseWriter, r *http.Request) {
		if !authorizedGatewayRuntimeRequest(r, authToken) {
			writeGatewayRuntimeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ownerAgentID, taskID, action, ok := parseGatewayRuntimeTaskPath(r.URL.Path)
		if !ok {
			writeGatewayRuntimeError(w, http.StatusNotFound, "not found")
			return
		}

		switch {
		case r.Method == http.MethodGet && action == "":
			task, err := agentLoop.GetRuntimeTask(ownerAgentID, taskID)
			if err != nil {
				writeGatewayRuntimeTaskError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(task)
		case r.Method == http.MethodPost && action == "cancel":
			task, err := agentLoop.CancelRuntimeTask(ownerAgentID, taskID)
			if err != nil {
				writeGatewayRuntimeTaskError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(task)
		case r.Method == http.MethodPost && action == "approve":
			var req runtimeReviewActionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
				writeGatewayRuntimeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			task, err := agentLoop.ApproveRuntimeTask(ownerAgentID, taskID, req.Actor, req.Note)
			if err != nil {
				writeGatewayRuntimeTaskError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(task)
		case r.Method == http.MethodPost && action == "reject":
			var req runtimeReviewActionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
				writeGatewayRuntimeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			task, err := agentLoop.RejectRuntimeTask(ownerAgentID, taskID, req.Actor, req.Note)
			if err != nil {
				writeGatewayRuntimeTaskError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(task)
		case r.Method == http.MethodPost && action == "memory-proposals":
			var req runtimeTaskMemoryProposalRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
				writeGatewayRuntimeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			proposal, err := agentLoop.CreateRuntimeMemoryProposalFromTask(ownerAgentID, taskID, req.Scope)
			if err != nil {
				writeGatewayRuntimeTaskError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(proposal)
		case action == "":
			writeGatewayRuntimeError(w, http.StatusMethodNotAllowed, "method not allowed")
		default:
			writeGatewayRuntimeError(w, http.StatusNotFound, "not found")
		}
	})

	channelManager.RegisterHTTPHandlerFunc(gatewayRuntimeMemoryProposalsPrefix, func(w http.ResponseWriter, r *http.Request) {
		if !authorizedGatewayRuntimeRequest(r, authToken) {
			writeGatewayRuntimeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ownerAgentID, proposalID, action, ok := parseGatewayRuntimeMemoryProposalPath(r.URL.Path)
		if !ok {
			writeGatewayRuntimeError(w, http.StatusNotFound, "not found")
			return
		}

		switch {
		case r.Method == http.MethodPost && action == "update":
			var req runtimeMemoryProposalUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
				writeGatewayRuntimeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			proposal, err := agentLoop.UpdateRuntimeMemoryProposal(ownerAgentID, proposalID, req.Actor, agent.MemoryProposalUpdate{
				Scope:   req.Scope,
				Title:   req.Title,
				Content: req.Content,
			})
			if err != nil {
				writeGatewayRuntimeTaskError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(proposal)
		case r.Method == http.MethodPost && action == "approve":
			var req runtimeReviewActionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
				writeGatewayRuntimeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			proposal, err := agentLoop.ApproveRuntimeMemoryProposal(ownerAgentID, proposalID, req.Actor, req.Note)
			if err != nil {
				writeGatewayRuntimeTaskError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(proposal)
		case r.Method == http.MethodPost && action == "reject":
			var req runtimeReviewActionRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
				writeGatewayRuntimeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			proposal, err := agentLoop.RejectRuntimeMemoryProposal(ownerAgentID, proposalID, req.Actor, req.Note)
			if err != nil {
				writeGatewayRuntimeTaskError(w, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(proposal)
		default:
			writeGatewayRuntimeError(w, http.StatusNotFound, "not found")
		}
	})
}

type httpHandlerRegistrar interface {
	RegisterHTTPHandlerFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

func authorizedGatewayRuntimeRequest(r *http.Request, authToken string) bool {
	authToken = strings.TrimSpace(authToken)
	if authToken == "" {
		return true
	}
	given := extractGatewayBearerToken(r.Header.Get("Authorization"))
	return given != "" && subtle.ConstantTimeCompare([]byte(given), []byte(authToken)) == 1
}

func extractGatewayBearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) < len(prefix) || header[:len(prefix)] != prefix {
		return ""
	}
	return header[len(prefix):]
}

func parseGatewayRuntimeTaskPath(path string) (ownerAgentID, taskID, action string, ok bool) {
	trimmed := strings.TrimPrefix(path, gatewayRuntimeTasksPrefix)
	if trimmed == path {
		return "", "", "", false
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 {
		return "", "", "", false
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2], true
	}
	return parts[0], parts[1], "", true
}

func parseGatewayRuntimeMemoryProposalPath(path string) (ownerAgentID, proposalID, action string, ok bool) {
	trimmed := strings.TrimPrefix(path, gatewayRuntimeMemoryProposalsPrefix)
	if trimmed == path {
		return "", "", "", false
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

func writeGatewayRuntimeTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agent.ErrRuntimeTaskNotFound):
		writeGatewayRuntimeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, agent.ErrRuntimeTaskNotCancelable):
		writeGatewayRuntimeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, agent.ErrRuntimeTaskNotAwaitingApproval):
		writeGatewayRuntimeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, agent.ErrRuntimeMemoryProposalNotFound):
		writeGatewayRuntimeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, agent.ErrRuntimeMemoryProposalNotPending):
		writeGatewayRuntimeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, agent.ErrRuntimeMemoryProposalInvalid):
		writeGatewayRuntimeError(w, http.StatusBadRequest, err.Error())
	default:
		writeGatewayRuntimeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeGatewayRuntimeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
