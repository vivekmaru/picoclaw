package agent

import (
	"errors"
	"fmt"
)

var (
	ErrRuntimeTaskNotFound      = errors.New("runtime task not found")
	ErrRuntimeTaskNotCancelable = errors.New("runtime task not cancelable")
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
