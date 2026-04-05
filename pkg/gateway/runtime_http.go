package gateway

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/agent"
)

const gatewayRuntimePath = "/runtime/agent"
const gatewayRuntimeTasksPrefix = "/runtime/agent/tasks/"

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
		case action == "":
			writeGatewayRuntimeError(w, http.StatusMethodNotAllowed, "method not allowed")
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

func writeGatewayRuntimeTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agent.ErrRuntimeTaskNotFound):
		writeGatewayRuntimeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, agent.ErrRuntimeTaskNotCancelable):
		writeGatewayRuntimeError(w, http.StatusConflict, err.Error())
	default:
		writeGatewayRuntimeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeGatewayRuntimeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
