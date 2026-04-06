package agent

import (
	"slices"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/tools"
)

type RuntimeSnapshot struct {
	GeneratedAt     int64                       `json:"generated_at"`
	Summary         RuntimeSnapshotSummary      `json:"summary"`
	Agents          []RuntimeAgentInfo          `json:"agents"`
	Teammates       []RuntimeTeammateInfo       `json:"teammates"`
	Tasks           []RuntimeTaskInfo           `json:"tasks"`
	MemoryProposals []RuntimeMemoryProposalInfo `json:"memory_proposals,omitempty"`
}

type RuntimeSnapshotSummary struct {
	AgentCount           int            `json:"agent_count"`
	TeammateCount        int            `json:"teammate_count"`
	TaskCount            int            `json:"task_count"`
	TaskStatuses         map[string]int `json:"task_statuses,omitempty"`
	MemoryProposalCount  int            `json:"memory_proposal_count"`
	MemoryProposalStatus map[string]int `json:"memory_proposal_statuses,omitempty"`
}

type RuntimeAgentInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Model     string `json:"model,omitempty"`
	Workspace string `json:"workspace,omitempty"`
}

type RuntimeTeammateInfo struct {
	ID             string   `json:"id"`
	Name           string   `json:"name,omitempty"`
	Role           string   `json:"role,omitempty"`
	AgentID        string   `json:"agent_id,omitempty"`
	Model          string   `json:"model,omitempty"`
	MemoryScope    string   `json:"memory_scope,omitempty"`
	ApprovalPolicy string   `json:"approval_policy,omitempty"`
	WorkspaceScope []string `json:"workspace_scope,omitempty"`
	Toolset        []string `json:"toolset,omitempty"`
}

type RuntimeTaskInfo struct {
	OwnerAgentID string `json:"owner_agent_id"`
	Cancelable   bool   `json:"cancelable,omitempty"`
	Approvable   bool   `json:"approvable,omitempty"`
	Rejectable   bool   `json:"rejectable,omitempty"`
	Handoffable  bool   `json:"handoffable,omitempty"`
	tools.SubagentTask
}

type RuntimeMemoryProposalInfo struct {
	OwnerAgentID string `json:"owner_agent_id"`
	Approvable   bool   `json:"approvable,omitempty"`
	Rejectable   bool   `json:"rejectable,omitempty"`
	MemoryProposal
}

type subagentManagerProvider interface {
	Manager() *tools.SubagentManager
}

func (al *AgentLoop) GetRuntimeSnapshot() RuntimeSnapshot {
	snapshot := RuntimeSnapshot{
		GeneratedAt: time.Now().UnixMilli(),
		Summary: RuntimeSnapshotSummary{
			TaskStatuses:         map[string]int{},
			MemoryProposalStatus: map[string]int{},
		},
	}

	registry := al.GetRegistry()
	if registry == nil {
		return snapshot
	}

	agentIDs := registry.ListAgentIDs()
	slices.Sort(agentIDs)
	snapshot.Agents = make([]RuntimeAgentInfo, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		agentInst, ok := registry.GetAgent(agentID)
		if !ok || agentInst == nil {
			continue
		}
		snapshot.Agents = append(snapshot.Agents, RuntimeAgentInfo{
			ID:        agentInst.ID,
			Name:      agentInst.Name,
			Model:     agentInst.Model,
			Workspace: agentInst.Workspace,
		})
	}

	teammates := registry.ListTeammates()
	snapshot.Teammates = make([]RuntimeTeammateInfo, 0, len(teammates))
	for _, teammate := range teammates {
		snapshot.Teammates = append(snapshot.Teammates, RuntimeTeammateInfo{
			ID:             teammate.ID,
			Name:           teammate.Name,
			Role:           teammate.Role,
			AgentID:        teammate.AgentID,
			Model:          teammate.Model,
			MemoryScope:    teammate.MemoryScope,
			ApprovalPolicy: teammate.ApprovalPolicy,
			WorkspaceScope: append([]string(nil), teammate.WorkspaceScope...),
			Toolset:        append([]string(nil), teammate.Toolset...),
		})
	}

	snapshot.Tasks = collectRuntimeTasks(registry, agentIDs)
	for _, task := range snapshot.Tasks {
		status := strings.TrimSpace(task.Status)
		if status == "" {
			status = "unknown"
		}
		snapshot.Summary.TaskStatuses[status]++
	}

	snapshot.MemoryProposals = collectRuntimeMemoryProposals(registry, agentIDs)
	for _, proposal := range snapshot.MemoryProposals {
		status := strings.TrimSpace(proposal.Status)
		if status == "" {
			status = "unknown"
		}
		snapshot.Summary.MemoryProposalStatus[status]++
	}

	snapshot.Summary.AgentCount = len(snapshot.Agents)
	snapshot.Summary.TeammateCount = len(snapshot.Teammates)
	snapshot.Summary.TaskCount = len(snapshot.Tasks)
	snapshot.Summary.MemoryProposalCount = len(snapshot.MemoryProposals)
	if len(snapshot.Summary.TaskStatuses) == 0 {
		snapshot.Summary.TaskStatuses = nil
	}
	if len(snapshot.Summary.MemoryProposalStatus) == 0 {
		snapshot.Summary.MemoryProposalStatus = nil
	}

	return snapshot
}

func collectRuntimeTasks(registry *AgentRegistry, agentIDs []string) []RuntimeTaskInfo {
	tasks := make([]RuntimeTaskInfo, 0)
	for _, agentID := range agentIDs {
		agentInst, ok := registry.GetAgent(agentID)
		if !ok || agentInst == nil || agentInst.Tools == nil {
			continue
		}

		var manager *tools.SubagentManager
		if rawTool, ok := agentInst.Tools.Get("spawn_status"); ok {
			if provider, ok := rawTool.(subagentManagerProvider); ok {
				manager = provider.Manager()
			}
		}
		if manager == nil {
			if rawTool, ok := agentInst.Tools.Get("spawn"); ok {
				if provider, ok := rawTool.(subagentManagerProvider); ok {
					manager = provider.Manager()
				}
			}
		}
		if manager == nil {
			continue
		}

		for _, task := range manager.ListTaskCopies() {
			tasks = append(tasks, runtimeTaskInfo(agentID, task))
		}
	}

	slices.SortFunc(tasks, func(a, b RuntimeTaskInfo) int {
		if a.Created != b.Created {
			if a.Created < b.Created {
				return -1
			}
			return 1
		}
		if a.OwnerAgentID != b.OwnerAgentID {
			if a.OwnerAgentID < b.OwnerAgentID {
				return -1
			}
			return 1
		}
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})

	return tasks
}

func runtimeTaskInfo(ownerAgentID string, task tools.SubagentTask) RuntimeTaskInfo {
	return RuntimeTaskInfo{
		OwnerAgentID: ownerAgentID,
		Cancelable:   runtimeTaskCancelable(task),
		Approvable:   runtimeTaskApprovable(task),
		Rejectable:   runtimeTaskRejectable(task),
		Handoffable:  runtimeTaskHandoffable(task),
		SubagentTask: task,
	}
}

func runtimeTaskCancelable(task tools.SubagentTask) bool {
	switch strings.ToLower(strings.TrimSpace(task.Status)) {
	case "awaiting_approval", "queued", "running":
		return true
	default:
		return false
	}
}

func runtimeTaskApprovable(task tools.SubagentTask) bool {
	return strings.EqualFold(strings.TrimSpace(task.Status), "awaiting_approval")
}

func runtimeTaskRejectable(task tools.SubagentTask) bool {
	return strings.EqualFold(strings.TrimSpace(task.Status), "awaiting_approval")
}

func runtimeTaskHandoffable(task tools.SubagentTask) bool {
	return isSubagentTaskTerminalStatus(task.Status)
}

func collectRuntimeMemoryProposals(registry *AgentRegistry, agentIDs []string) []RuntimeMemoryProposalInfo {
	proposals := make([]RuntimeMemoryProposalInfo, 0)
	for _, agentID := range agentIDs {
		store := runtimeMemoryProposalStoreForAgent(registry, agentID)
		if store == nil {
			continue
		}
		for _, proposal := range store.ListCopies() {
			proposals = append(proposals, runtimeMemoryProposalInfo(agentID, proposal))
		}
	}
	slices.SortFunc(proposals, func(a, b RuntimeMemoryProposalInfo) int {
		if a.Created != b.Created {
			if a.Created < b.Created {
				return -1
			}
			return 1
		}
		if a.OwnerAgentID != b.OwnerAgentID {
			if a.OwnerAgentID < b.OwnerAgentID {
				return -1
			}
			return 1
		}
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	return proposals
}

func runtimeMemoryProposalInfo(ownerAgentID string, proposal MemoryProposal) RuntimeMemoryProposalInfo {
	return RuntimeMemoryProposalInfo{
		OwnerAgentID:   ownerAgentID,
		Approvable:     strings.EqualFold(strings.TrimSpace(proposal.Status), "pending"),
		Rejectable:     strings.EqualFold(strings.TrimSpace(proposal.Status), "pending"),
		MemoryProposal: proposal,
	}
}

func runtimeTaskManagerForAgent(registry *AgentRegistry, agentID string) *tools.SubagentManager {
	if registry == nil {
		return nil
	}
	agentInst, ok := registry.GetAgent(agentID)
	if !ok || agentInst == nil || agentInst.Tools == nil {
		return nil
	}

	if rawTool, ok := agentInst.Tools.Get("spawn_status"); ok {
		if provider, ok := rawTool.(subagentManagerProvider); ok {
			return provider.Manager()
		}
	}
	if rawTool, ok := agentInst.Tools.Get("spawn"); ok {
		if provider, ok := rawTool.(subagentManagerProvider); ok {
			return provider.Manager()
		}
	}
	return nil
}

func runtimeMemoryProposalStoreForAgent(registry *AgentRegistry, agentID string) *MemoryProposalStore {
	if registry == nil {
		return nil
	}
	agentInst, ok := registry.GetAgent(agentID)
	if !ok || agentInst == nil || strings.TrimSpace(agentInst.Workspace) == "" {
		return nil
	}
	return NewMemoryProposalStore(agentInst.Workspace)
}
