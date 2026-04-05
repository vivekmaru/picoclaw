package agent

import (
	"slices"
	"sync"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/routing"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// AgentRegistry manages multiple agent instances and routes messages to them.
type AgentRegistry struct {
	agents    map[string]*AgentInstance
	teammates map[string]TeammateProfile
	resolver  *routing.RouteResolver
	mu        sync.RWMutex
}

// NewAgentRegistry creates a registry from config, instantiating all agents.
func NewAgentRegistry(
	cfg *config.Config,
	provider providers.LLMProvider,
) *AgentRegistry {
	registry := &AgentRegistry{
		agents:    make(map[string]*AgentInstance),
		teammates: make(map[string]TeammateProfile),
		resolver:  routing.NewRouteResolver(cfg),
	}

	agentConfigs := cfg.Agents.List
	if len(agentConfigs) == 0 {
		implicitAgent := &config.AgentConfig{
			ID:      "main",
			Default: true,
		}
		instance := NewAgentInstance(implicitAgent, &cfg.Agents.Defaults, cfg, provider)
		registry.agents["main"] = instance
		logger.InfoCF("agent", "Created implicit main agent (no agents.list configured)", nil)
	} else {
		for i := range agentConfigs {
			ac := &agentConfigs[i]
			id := routing.NormalizeAgentID(ac.ID)
			instance := NewAgentInstance(ac, &cfg.Agents.Defaults, cfg, provider)
			registry.agents[id] = instance
			logger.InfoCF("agent", "Registered agent",
				map[string]any{
					"agent_id":  id,
					"name":      ac.Name,
					"workspace": instance.Workspace,
					"model":     instance.Model,
				})
		}
	}

	registry.teammates = buildTeammateProfiles(cfg, registry.agents)

	return registry
}

// GetAgent returns the agent instance for a given ID.
func (r *AgentRegistry) GetAgent(agentID string) (*AgentInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := routing.NormalizeAgentID(agentID)
	agent, ok := r.agents[id]
	return agent, ok
}

// ResolveRoute determines which agent handles the message.
func (r *AgentRegistry) ResolveRoute(input routing.RouteInput) routing.ResolvedRoute {
	return r.resolver.ResolveRoute(input)
}

// ListAgentIDs returns all registered agent IDs.
func (r *AgentRegistry) ListAgentIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}
	return ids
}

func (r *AgentRegistry) GetTeammate(teammateID string) (TeammateProfile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := routing.NormalizeAgentID(teammateID)
	profile, ok := r.teammates[id]
	return profile, ok
}

func (r *AgentRegistry) ListTeammateIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.teammates))
	for id := range r.teammates {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func (r *AgentRegistry) ListTeammates() []TeammateProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.teammates))
	for id := range r.teammates {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	profiles := make([]TeammateProfile, 0, len(ids))
	for _, id := range ids {
		profiles = append(profiles, r.teammates[id])
	}
	return profiles
}

func (r *AgentRegistry) DefaultTeammateForAgent(agentID string) (TeammateProfile, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	normAgentID := routing.NormalizeAgentID(agentID)
	if profile, ok := r.teammates[normAgentID]; ok {
		return profile, true
	}
	ids := make([]string, 0, len(r.teammates))
	for id, profile := range r.teammates {
		if routing.NormalizeAgentID(profile.AgentID) == normAgentID {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return TeammateProfile{}, false
	}
	slices.Sort(ids)
	return r.teammates[ids[0]], true
}

// CanSpawnSubagent checks if parentAgentID is allowed to spawn targetAgentID.
func (r *AgentRegistry) CanSpawnSubagent(parentAgentID, targetAgentID string) bool {
	parent, ok := r.GetAgent(parentAgentID)
	if !ok {
		return false
	}
	if parent.Subagents == nil || parent.Subagents.AllowAgents == nil {
		return false
	}
	targetNorm := routing.NormalizeAgentID(targetAgentID)
	for _, allowed := range parent.Subagents.AllowAgents {
		if allowed == "*" {
			return true
		}
		if routing.NormalizeAgentID(allowed) == targetNorm {
			return true
		}
	}
	return false
}

func (r *AgentRegistry) CanDelegateToTeammate(parentAgentID, teammateID string) bool {
	profile, ok := r.GetTeammate(teammateID)
	if !ok {
		return false
	}
	return r.CanSpawnSubagent(parentAgentID, profile.AgentID)
}

// ForEachTool calls fn for every tool registered under the given name
// across all agents. This is useful for propagating dependencies (e.g.
// MediaStore) to tools after registry construction.
func (r *AgentRegistry) ForEachTool(name string, fn func(tools.Tool)) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, agent := range r.agents {
		if t, ok := agent.Tools.Get(name); ok {
			fn(t)
		}
	}
}

// Close releases resources held by all registered agents.
func (r *AgentRegistry) Close() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, agent := range r.agents {
		if err := agent.Close(); err != nil {
			logger.WarnCF("agent", "Failed to close agent",
				map[string]any{"agent_id": agent.ID, "error": err.Error()})
		}
	}
}

// GetDefaultAgent returns the default agent instance.
func (r *AgentRegistry) GetDefaultAgent() *AgentInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if agent, ok := r.agents["main"]; ok {
		return agent
	}
	for _, agent := range r.agents {
		return agent
	}
	return nil
}
