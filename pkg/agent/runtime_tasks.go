package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sipeed/picoclaw/pkg/tools"
)

var (
	ErrRuntimeTaskNotFound             = errors.New("runtime task not found")
	ErrRuntimeTaskNotCancelable        = errors.New("runtime task not cancelable")
	ErrRuntimeTaskNotAwaitingApproval  = errors.New("runtime task not awaiting approval")
	ErrRuntimeTaskNotHandoffable       = errors.New("runtime task not handoffable")
	ErrRuntimeTaskInvalid              = errors.New("runtime task invalid")
	ErrRuntimeMemoryProposalNotFound   = errors.New("runtime memory proposal not found")
	ErrRuntimeMemoryProposalNotPending = errors.New("runtime memory proposal not pending")
	ErrRuntimeMemoryProposalInvalid    = errors.New("runtime memory proposal invalid")
)

type RuntimeTaskHandoffRequest struct {
	Actor      string `json:"actor,omitempty"`
	Note       string `json:"note,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	TeammateID string `json:"teammate_id,omitempty"`
	Label      string `json:"label,omitempty"`
	Task       string `json:"task,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

func (al *AgentLoop) GetRuntimeTask(ownerAgentID, taskID string) (RuntimeTaskInfo, error) {
	if al == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: agent loop unavailable", ErrRuntimeTaskNotFound)
	}
	registry := al.GetRegistry()
	manager := runtimeTaskManagerForAgent(registry, ownerAgentID)
	if manager == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: owner agent %q", ErrRuntimeTaskNotFound, ownerAgentID)
	}

	task, ok := manager.GetTaskCopy(taskID)
	if !ok {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: %s", ErrRuntimeTaskNotFound, taskID)
	}
	return runtimeTaskInfo(ownerAgentID, task), nil
}

func (al *AgentLoop) CancelRuntimeTask(ownerAgentID, taskID string) (RuntimeTaskInfo, error) {
	if al == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: agent loop unavailable", ErrRuntimeTaskNotFound)
	}
	registry := al.GetRegistry()
	manager := runtimeTaskManagerForAgent(registry, ownerAgentID)
	if manager == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: owner agent %q", ErrRuntimeTaskNotFound, ownerAgentID)
	}

	task, err := manager.CancelTask(taskID)
	if err != nil {
		if _, ok := manager.GetTaskCopy(taskID); !ok {
			return RuntimeTaskInfo{}, fmt.Errorf("%w: %s", ErrRuntimeTaskNotFound, taskID)
		}
		return RuntimeTaskInfo{}, fmt.Errorf("%w: %v", ErrRuntimeTaskNotCancelable, err)
	}
	return runtimeTaskInfo(ownerAgentID, task), nil
}

func (al *AgentLoop) ApproveRuntimeTask(ownerAgentID, taskID, actor, note string) (RuntimeTaskInfo, error) {
	if al == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: agent loop unavailable", ErrRuntimeTaskNotFound)
	}
	registry := al.GetRegistry()
	manager := runtimeTaskManagerForAgent(registry, ownerAgentID)
	if manager == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: owner agent %q", ErrRuntimeTaskNotFound, ownerAgentID)
	}
	task, err := manager.ApproveTask(taskID, actor, note)
	if err != nil {
		if _, ok := manager.GetTaskCopy(taskID); !ok {
			return RuntimeTaskInfo{}, fmt.Errorf("%w: %s", ErrRuntimeTaskNotFound, taskID)
		}
		return RuntimeTaskInfo{}, fmt.Errorf("%w: %v", ErrRuntimeTaskNotAwaitingApproval, err)
	}
	return runtimeTaskInfo(ownerAgentID, task), nil
}

func (al *AgentLoop) RejectRuntimeTask(ownerAgentID, taskID, actor, note string) (RuntimeTaskInfo, error) {
	if al == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: agent loop unavailable", ErrRuntimeTaskNotFound)
	}
	registry := al.GetRegistry()
	manager := runtimeTaskManagerForAgent(registry, ownerAgentID)
	if manager == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: owner agent %q", ErrRuntimeTaskNotFound, ownerAgentID)
	}
	task, err := manager.RejectTask(taskID, actor, note)
	if err != nil {
		if _, ok := manager.GetTaskCopy(taskID); !ok {
			return RuntimeTaskInfo{}, fmt.Errorf("%w: %s", ErrRuntimeTaskNotFound, taskID)
		}
		return RuntimeTaskInfo{}, fmt.Errorf("%w: %v", ErrRuntimeTaskNotAwaitingApproval, err)
	}
	return runtimeTaskInfo(ownerAgentID, task), nil
}

func (al *AgentLoop) HandoffRuntimeTask(
	ownerAgentID, taskID string,
	req RuntimeTaskHandoffRequest,
) (RuntimeTaskInfo, error) {
	sourceTask, err := al.GetRuntimeTask(ownerAgentID, taskID)
	if err != nil {
		return RuntimeTaskInfo{}, err
	}
	if !runtimeTaskHandoffable(sourceTask.SubagentTask) {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: task %s is not in a terminal state", ErrRuntimeTaskNotHandoffable, taskID)
	}

	registry := al.GetRegistry()
	if registry == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: agent registry unavailable", ErrRuntimeTaskNotFound)
	}

	targetTeammateID := strings.TrimSpace(req.TeammateID)
	targetAgentID := strings.TrimSpace(req.AgentID)
	if targetTeammateID == "" && targetAgentID == "" {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: handoff target teammate_id or agent_id is required", ErrRuntimeTaskInvalid)
	}

	if targetTeammateID != "" {
		teammate, ok := registry.GetTeammate(targetTeammateID)
		if !ok {
			return RuntimeTaskInfo{}, fmt.Errorf("%w: teammate %q not found", ErrRuntimeTaskInvalid, targetTeammateID)
		}
		targetAgentID = teammate.AgentID
	} else {
		if _, ok := registry.GetAgent(targetAgentID); !ok {
			return RuntimeTaskInfo{}, fmt.Errorf("%w: agent %q not found", ErrRuntimeTaskInvalid, targetAgentID)
		}
	}

	manager := runtimeTaskManagerForAgent(registry, targetAgentID)
	if manager == nil {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: owner agent %q", ErrRuntimeTaskNotFound, targetAgentID)
	}

	handoffKind := strings.TrimSpace(req.Kind)
	if handoffKind == "" {
		if strings.EqualFold(targetTeammateID, "reviewer") {
			handoffKind = "review"
		} else {
			handoffKind = "follow_up"
		}
	}

	taskText := strings.TrimSpace(req.Task)
	if taskText == "" {
		taskText = buildRuntimeTaskHandoffPrompt(sourceTask, handoffKind, strings.TrimSpace(req.Note))
	}
	if taskText == "" {
		return RuntimeTaskInfo{}, fmt.Errorf("%w: handoff task is required", ErrRuntimeTaskInvalid)
	}

	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = buildRuntimeTaskHandoffLabel(sourceTask, handoffKind)
	}

	requesterTeammateID := strings.TrimSpace(sourceTask.TeammateID)
	if requesterTeammateID == "" {
		requesterTeammateID = strings.TrimSpace(sourceTask.RequesterTeammateID)
	}

	childTask, err := manager.Spawn(context.Background(), tools.SpawnRequest{
		Task:                taskText,
		Label:               label,
		AgentID:             targetAgentID,
		TeammateID:          targetTeammateID,
		RequesterAgentID:    ownerAgentID,
		RequesterTeammateID: requesterTeammateID,
		ParentTaskID:        sourceTask.ID,
		ParentOwnerAgentID:  ownerAgentID,
		RootTaskID:          firstNonEmpty(strings.TrimSpace(sourceTask.RootTaskID), sourceTask.ID),
		RootOwnerAgentID:    firstNonEmpty(strings.TrimSpace(sourceTask.RootOwnerAgentID), ownerAgentID),
		HandoffKind:         handoffKind,
		HandoffActor:        strings.TrimSpace(req.Actor),
		HandoffNote:         strings.TrimSpace(req.Note),
	}, nil)
	if err != nil {
		return RuntimeTaskInfo{}, err
	}
	return runtimeTaskInfo(targetAgentID, childTask), nil
}

func (al *AgentLoop) CreateRuntimeMemoryProposalFromTask(ownerAgentID, taskID, scope string) (RuntimeMemoryProposalInfo, error) {
	task, err := al.GetRuntimeTask(ownerAgentID, taskID)
	if err != nil {
		return RuntimeMemoryProposalInfo{}, err
	}
	if !isSubagentTaskTerminalStatus(task.Status) {
		return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: task %s is not in a terminal state", ErrRuntimeTaskNotCancelable, taskID)
	}
	content := strings.TrimSpace(task.Result)
	if content == "" {
		return RuntimeMemoryProposalInfo{}, fmt.Errorf("task %q has no result to promote", taskID)
	}
	registry := al.GetRegistry()
	store := runtimeMemoryProposalStoreForAgent(registry, ownerAgentID)
	if store == nil {
		return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: owner agent %q", ErrRuntimeMemoryProposalNotFound, ownerAgentID)
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = task.MemoryScope
	}
	if scope == "" {
		scope = "shared"
	}
	title := task.Label
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("Task Memory: %s", task.ID)
	}
	proposal, err := store.Create(MemoryProposalRequest{
		Scope:               scope,
		Target:              "long_term",
		Kind:                "task_result",
		Title:               title,
		Content:             content,
		SourceTaskID:        task.ID,
		SourceAgentID:       ownerAgentID,
		SourceTeammateID:    task.TeammateID,
		RequesterAgentID:    task.RequesterAgentID,
		RequesterTeammateID: task.RequesterTeammateID,
	})
	if err != nil {
		return RuntimeMemoryProposalInfo{}, err
	}
	return runtimeMemoryProposalInfo(ownerAgentID, proposal), nil
}

func (al *AgentLoop) UpdateRuntimeMemoryProposal(
	ownerAgentID, proposalID, actor string,
	update MemoryProposalUpdate,
) (RuntimeMemoryProposalInfo, error) {
	registry := al.GetRegistry()
	store := runtimeMemoryProposalStoreForAgent(registry, ownerAgentID)
	if store == nil {
		return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: owner agent %q", ErrRuntimeMemoryProposalNotFound, ownerAgentID)
	}
	proposal, err := store.Update(proposalID, actor, update)
	if err != nil {
		if _, ok := store.GetCopy(proposalID); !ok {
			return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: %s", ErrRuntimeMemoryProposalNotFound, proposalID)
		}
		if errors.Is(err, errMemoryProposalNotPending) {
			return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: %v", ErrRuntimeMemoryProposalNotPending, err)
		}
		if errors.Is(err, errMemoryProposalInvalid) {
			return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: %v", ErrRuntimeMemoryProposalInvalid, err)
		}
		return RuntimeMemoryProposalInfo{}, err
	}
	return runtimeMemoryProposalInfo(ownerAgentID, proposal), nil
}

func (al *AgentLoop) ApproveRuntimeMemoryProposal(ownerAgentID, proposalID, actor, note string) (RuntimeMemoryProposalInfo, error) {
	registry := al.GetRegistry()
	store := runtimeMemoryProposalStoreForAgent(registry, ownerAgentID)
	if store == nil {
		return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: owner agent %q", ErrRuntimeMemoryProposalNotFound, ownerAgentID)
	}
	proposal, err := store.Approve(proposalID, actor, note)
	if err != nil {
		if _, ok := store.GetCopy(proposalID); !ok {
			return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: %s", ErrRuntimeMemoryProposalNotFound, proposalID)
		}
		if errors.Is(err, errMemoryProposalNotPending) {
			return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: %v", ErrRuntimeMemoryProposalNotPending, err)
		}
		return RuntimeMemoryProposalInfo{}, err
	}
	return runtimeMemoryProposalInfo(ownerAgentID, proposal), nil
}

func (al *AgentLoop) RejectRuntimeMemoryProposal(ownerAgentID, proposalID, actor, note string) (RuntimeMemoryProposalInfo, error) {
	registry := al.GetRegistry()
	store := runtimeMemoryProposalStoreForAgent(registry, ownerAgentID)
	if store == nil {
		return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: owner agent %q", ErrRuntimeMemoryProposalNotFound, ownerAgentID)
	}
	proposal, err := store.Reject(proposalID, actor, note)
	if err != nil {
		if _, ok := store.GetCopy(proposalID); !ok {
			return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: %s", ErrRuntimeMemoryProposalNotFound, proposalID)
		}
		if errors.Is(err, errMemoryProposalNotPending) {
			return RuntimeMemoryProposalInfo{}, fmt.Errorf("%w: %v", ErrRuntimeMemoryProposalNotPending, err)
		}
		return RuntimeMemoryProposalInfo{}, err
	}
	return runtimeMemoryProposalInfo(ownerAgentID, proposal), nil
}

func isSubagentTaskTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed", "canceled", "denied":
		return true
	default:
		return false
	}
}

func buildRuntimeTaskHandoffLabel(sourceTask RuntimeTaskInfo, handoffKind string) string {
	base := strings.TrimSpace(sourceTask.Label)
	if base == "" {
		base = sourceTask.ID
	}
	switch strings.TrimSpace(handoffKind) {
	case "review":
		return "Review: " + base
	default:
		return "Follow-up: " + base
	}
}

func buildRuntimeTaskHandoffPrompt(sourceTask RuntimeTaskInfo, handoffKind, note string) string {
	var sb strings.Builder
	switch strings.TrimSpace(handoffKind) {
	case "review":
		sb.WriteString("Review the completed task below and provide feedback, risks, or next steps.")
	default:
		sb.WriteString("Continue from the completed task below and handle the requested follow-up work.")
	}
	sb.WriteString("\n\n")
	sb.WriteString("Source task ID: ")
	sb.WriteString(sourceTask.ID)
	if sourceTask.Label != "" {
		sb.WriteString("\nSource label: ")
		sb.WriteString(sourceTask.Label)
	}
	if sourceTask.TeammateID != "" {
		sb.WriteString("\nSource teammate: ")
		sb.WriteString(sourceTask.TeammateID)
	}
	sb.WriteString("\nSource status: ")
	sb.WriteString(sourceTask.Status)
	sb.WriteString("\n\nOriginal task:\n")
	sb.WriteString(strings.TrimSpace(sourceTask.Task))
	if result := strings.TrimSpace(sourceTask.Result); result != "" {
		sb.WriteString("\n\nResult:\n")
		sb.WriteString(result)
	}
	if trimmedNote := strings.TrimSpace(note); trimmedNote != "" {
		sb.WriteString("\n\nHandoff note:\n")
		sb.WriteString(trimmedNote)
	}
	return strings.TrimSpace(sb.String())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
