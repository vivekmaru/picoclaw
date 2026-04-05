package agent

import (
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/routing"
	"github.com/sipeed/picoclaw/pkg/tools"
)

type TeammateProfile struct {
	ID             string
	Name           string
	Role           string
	AgentID        string
	Model          string
	MemoryScope    string
	ApprovalPolicy string
	WorkspaceScope []string
	Toolset        []string
}

func buildTeammateProfiles(cfg *config.Config, agents map[string]*AgentInstance) map[string]TeammateProfile {
	profiles := make(map[string]TeammateProfile)
	coveredAgents := make(map[string]bool)

	for _, teammate := range cfg.Teammates.List {
		id := routing.NormalizeAgentID(teammate.ID)
		if id == "" {
			continue
		}
		agentID := routing.NormalizeAgentID(teammate.AgentID)
		if agentID == "" {
			agentID = id
		}
		agentInst, ok := agents[agentID]
		if !ok || agentInst == nil {
			continue
		}
		profiles[id] = resolveTeammateProfile(cfg, teammate, agentInst, id)
		coveredAgents[agentID] = true
	}

	for agentID, agentInst := range agents {
		if coveredAgents[agentID] {
			continue
		}
		implicit := config.TeammateConfig{
			ID:      agentID,
			Name:    agentInst.Name,
			AgentID: agentID,
		}
		profiles[agentID] = resolveTeammateProfile(cfg, implicit, agentInst, agentID)
	}

	return profiles
}

func resolveTeammateProfile(
	cfg *config.Config,
	teammate config.TeammateConfig,
	agentInst *AgentInstance,
	id string,
) TeammateProfile {
	role := strings.TrimSpace(teammate.Role)
	if role == "" {
		role = strings.TrimSpace(cfg.Teammates.Defaults.Role)
	}
	if role == "" {
		role = "general"
	}

	name := strings.TrimSpace(teammate.Name)
	if name == "" {
		name = strings.TrimSpace(agentInst.Name)
	}
	if name == "" {
		name = id
	}

	model := strings.TrimSpace(teammate.Model)
	if model == "" {
		model = agentInst.Model
	}

	memoryScope := strings.TrimSpace(teammate.MemoryScope)
	if memoryScope == "" {
		memoryScope = strings.TrimSpace(cfg.Teammates.Defaults.MemoryScope)
	}
	if memoryScope == "" {
		memoryScope = "teammate:" + id
	}

	approvalPolicy := strings.TrimSpace(teammate.ApprovalPolicy)
	if approvalPolicy == "" {
		approvalPolicy = strings.TrimSpace(cfg.Teammates.Defaults.ApprovalPolicy)
	}
	if approvalPolicy == "" {
		approvalPolicy = cfg.Trust.EffectiveApprovalPolicy()
	}

	workspaceScope := append([]string(nil), teammate.WorkspaceScope...)
	if len(workspaceScope) == 0 && agentInst.Workspace != "" {
		workspaceScope = []string{agentInst.Workspace}
	}

	toolset := append([]string(nil), teammate.Toolset...)

	return TeammateProfile{
		ID:             id,
		Name:           name,
		Role:           role,
		AgentID:        agentInst.ID,
		Model:          model,
		MemoryScope:    memoryScope,
		ApprovalPolicy: approvalPolicy,
		WorkspaceScope: workspaceScope,
		Toolset:        toolset,
	}
}

func (p TeammateProfile) toTaskTeammate() tools.TaskTeammate {
	return tools.TaskTeammate{
		ID:             p.ID,
		Name:           p.Name,
		Role:           p.Role,
		AgentID:        p.AgentID,
		Model:          p.Model,
		MemoryScope:    p.MemoryScope,
		ApprovalPolicy: p.ApprovalPolicy,
		WorkspaceScope: append([]string(nil), p.WorkspaceScope...),
		Toolset:        append([]string(nil), p.Toolset...),
	}
}
