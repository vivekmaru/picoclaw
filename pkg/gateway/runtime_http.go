package gateway

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sipeed/picoclaw/pkg/agent"
)

const gatewayRuntimePath = "/runtime/agent"

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
