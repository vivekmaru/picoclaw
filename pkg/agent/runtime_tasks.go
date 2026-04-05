package agent

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrRuntimeTaskNotFound             = errors.New("runtime task not found")
	ErrRuntimeTaskNotCancelable        = errors.New("runtime task not cancelable")
	ErrRuntimeTaskNotAwaitingApproval  = errors.New("runtime task not awaiting approval")
	ErrRuntimeMemoryProposalNotFound   = errors.New("runtime memory proposal not found")
	ErrRuntimeMemoryProposalNotPending = errors.New("runtime memory proposal not pending")
	ErrRuntimeMemoryProposalInvalid    = errors.New("runtime memory proposal invalid")
)

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
