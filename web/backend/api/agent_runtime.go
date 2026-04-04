package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
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

func (h *Handler) getGatewayRuntimeSnapshot(timeout time.Duration) (*agent.RuntimeSnapshot, int, error) {
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

	req, err := http.NewRequest(http.MethodGet, "http://"+net.JoinHostPort(host, strconv.Itoa(port))+"/runtime/agent", nil)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if pidData.Token != "" {
		req.Header.Set("Authorization", "Bearer "+pidData.Token)
	}

	resp, err := gatewayRuntimeDo(req, timeout)
	if err != nil {
		return nil, http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, http.StatusBadGateway, fmt.Errorf("gateway runtime endpoint returned %s", resp.Status)
	}

	var snapshot agent.RuntimeSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snapshot); err != nil {
		return nil, http.StatusBadGateway, err
	}
	return &snapshot, http.StatusOK, nil
}
