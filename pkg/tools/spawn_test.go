package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockSpawner implements SubTurnSpawner for testing
type mockSpawner struct{}

func (m *mockSpawner) SpawnSubTurn(ctx context.Context, cfg SubTurnConfig) (*ToolResult, error) {
	// Extract task from system prompt for response
	task := cfg.SystemPrompt
	if strings.Contains(task, "Task: ") {
		parts := strings.Split(task, "Task: ")
		if len(parts) > 1 {
			task = parts[1]
		}
	}
	return &ToolResult{
		ForLLM:  "Task completed: " + task,
		ForUser: "Task completed",
	}, nil
}

func TestSpawnTool_Execute_EmptyTask(t *testing.T) {
	provider := &MockLLMProvider{}
	manager := NewSubagentManager(provider, "test-model", "/tmp/test")
	tool := NewSpawnTool(manager)

	ctx := context.Background()

	tests := []struct {
		name string
		args map[string]any
	}{
		{"empty string", map[string]any{"task": ""}},
		{"whitespace only", map[string]any{"task": "   "}},
		{"tabs and newlines", map[string]any{"task": "\t\n  "}},
		{"missing task key", map[string]any{"label": "test"}},
		{"wrong type", map[string]any{"task": 123}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.Execute(ctx, tt.args)
			if result == nil {
				t.Fatal("Result should not be nil")
			}
			if !result.IsError {
				t.Error("Expected error for invalid task parameter")
			}
			if !strings.Contains(result.ForLLM, "task is required") {
				t.Errorf("Error message should mention 'task is required', got: %s", result.ForLLM)
			}
		})
	}
}

func TestSpawnTool_Execute_ValidTask(t *testing.T) {
	provider := &MockLLMProvider{}
	manager := NewSubagentManager(provider, "test-model", "/tmp/test")
	tool := NewSpawnTool(manager)
	tool.SetSpawner(&mockSpawner{})

	ctx := context.Background()
	args := map[string]any{
		"task":  "Write a haiku about coding",
		"label": "haiku-task",
	}

	result := tool.Execute(ctx, args)
	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.IsError {
		t.Errorf("Expected success for valid task, got error: %s", result.ForLLM)
	}
	if !result.Async {
		t.Error("SpawnTool should return async result")
	}
}

func TestSpawnTool_Execute_TrackedTaskRecord(t *testing.T) {
	provider := &MockLLMProvider{}
	manager := NewSubagentManager(provider, "test-model", "/tmp/test")
	manager.SetSpawner(func(
		ctx context.Context,
		task, label, agentID, teammateID string,
		tls *ToolRegistry,
		maxTokens int,
		temperature float64,
		hasMaxTokens, hasTemperature bool,
	) (*ToolResult, error) {
		return &ToolResult{ForLLM: "done"}, nil
	})
	manager.SetTeammateResolver(func(teammateID string) (TaskTeammate, bool) {
		if teammateID != "reviewer" {
			return TaskTeammate{}, false
		}
		return TaskTeammate{
			ID:          "reviewer",
			AgentID:     "coder",
			MemoryScope: "team/reviewer",
		}, true
	})
	tool := NewSpawnTool(manager)
	tool.SetRequesterIdentity("planner", "planner")

	result := tool.Execute(WithToolContext(context.Background(), "internal", "chat-1"), map[string]any{
		"task":        "Review the patch",
		"label":       "review",
		"teammate_id": "reviewer",
	})
	if result == nil || result.IsError {
		t.Fatalf("expected tracked spawn success, got %+v", result)
	}
	if !result.Async {
		t.Fatal("expected async tracked spawn result")
	}

	var task SubagentTask
	if err := json.Unmarshal([]byte(result.ForLLM), &task); err != nil {
		t.Fatalf("expected structured task payload, got %q error=%v", result.ForLLM, err)
	}
	if task.TeammateID != "reviewer" {
		t.Fatalf("TeammateID = %q, want reviewer", task.TeammateID)
	}
	if task.RequesterTeammateID != "planner" {
		t.Fatalf("RequesterTeammateID = %q, want planner", task.RequesterTeammateID)
	}
	if task.MemoryScope != "team/reviewer" {
		t.Fatalf("MemoryScope = %q, want team/reviewer", task.MemoryScope)
	}
}

func TestSpawnTool_Execute_NilManager(t *testing.T) {
	tool := NewSpawnTool(nil)

	ctx := context.Background()
	args := map[string]any{"task": "test task"}

	result := tool.Execute(ctx, args)
	if !result.IsError {
		t.Error("Expected error for nil manager")
	}
	if !strings.Contains(result.ForLLM, "Subagent manager not configured") {
		t.Errorf("Error message should mention manager not configured, got: %s", result.ForLLM)
	}
}

func TestSubagentManager_LoadPersistedTasksMarksInterrupted(t *testing.T) {
	workspace := t.TempDir()
	stateFile := filepath.Join(workspace, "state", "subagents", "tasks.json")
	storeData, err := json.MarshalIndent(subagentTaskStore{
		Version: subagentTaskStoreVersion,
		NextID:  4,
		Tasks: []SubagentTask{
			{ID: "subagent-1", Task: "queued work", Status: "queued", Created: 1},
			{ID: "subagent-2", Task: "running work", Status: "running", Created: 2},
			{ID: "subagent-3", Task: "done work", Status: "completed", Created: 3},
		},
	}, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(stateFile, storeData, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manager := NewSubagentManager(&MockLLMProvider{}, "test-model", workspace)

	task1, ok := manager.GetTaskCopy("subagent-1")
	if !ok {
		t.Fatal("expected queued task to be loaded")
	}
	if task1.Status != "failed" {
		t.Fatalf("task1.Status = %q, want failed", task1.Status)
	}
	if task1.Completed == 0 {
		t.Fatal("task1.Completed should be set during restart recovery")
	}

	task2, ok := manager.GetTaskCopy("subagent-2")
	if !ok {
		t.Fatal("expected running task to be loaded")
	}
	if task2.Status != "failed" {
		t.Fatalf("task2.Status = %q, want failed", task2.Status)
	}

	manager.SetSpawner(func(
		ctx context.Context,
		task, label, agentID, teammateID string,
		tls *ToolRegistry,
		maxTokens int,
		temperature float64,
		hasMaxTokens, hasTemperature bool,
	) (*ToolResult, error) {
		return &ToolResult{ForLLM: "done"}, nil
	})
	spawned, err := manager.Spawn(context.Background(), SpawnRequest{Task: "fresh task"}, nil)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	if spawned.ID != "subagent-4" {
		t.Fatalf("spawned.ID = %q, want subagent-4", spawned.ID)
	}
}

func TestSubagentManager_CancelTaskPersistsState(t *testing.T) {
	workspace := t.TempDir()
	manager := NewSubagentManager(&MockLLMProvider{}, "test-model", workspace)

	started := make(chan struct{})
	manager.SetSpawner(func(
		ctx context.Context,
		task, label, agentID, teammateID string,
		tls *ToolRegistry,
		maxTokens int,
		temperature float64,
		hasMaxTokens, hasTemperature bool,
	) (*ToolResult, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})

	task, err := manager.Spawn(context.Background(), SpawnRequest{Task: "long task"}, nil)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	<-started

	cancelled, err := manager.CancelTask(task.ID)
	if err != nil {
		t.Fatalf("CancelTask() error = %v", err)
	}
	if cancelled.Status != "canceling" {
		t.Fatalf("CancelTask() status = %q, want canceling", cancelled.Status)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		current, ok := manager.GetTaskCopy(task.ID)
		if !ok {
			t.Fatalf("task %s disappeared", task.ID)
		}
		if current.Status == "canceled" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s did not cancel, status=%s", task.ID, current.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	stateFile := filepath.Join(workspace, "state", "subagents", "tasks.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `"status": "canceled"`) {
		t.Fatalf("persisted task store missing canceled status: %s", string(data))
	}
}

func TestSubagentManager_ApprovalWorkflow(t *testing.T) {
	workspace := t.TempDir()
	manager := NewSubagentManager(&MockLLMProvider{}, "test-model", workspace)
	manager.SetTeammateResolver(func(teammateID string) (TaskTeammate, bool) {
		if teammateID != "operator" {
			return TaskTeammate{}, false
		}
		return TaskTeammate{
			ID:             "operator",
			AgentID:        "main",
			MemoryScope:    "teammate:operator",
			ApprovalPolicy: "confirm_exec",
		}, true
	})

	started := make(chan struct{})
	manager.SetSpawner(func(
		ctx context.Context,
		task, label, agentID, teammateID string,
		tls *ToolRegistry,
		maxTokens int,
		temperature float64,
		hasMaxTokens, hasTemperature bool,
	) (*ToolResult, error) {
		close(started)
		return &ToolResult{ForLLM: "approved execution"}, nil
	})

	task, err := manager.Spawn(context.Background(), SpawnRequest{
		Task:       "Restart the service",
		TeammateID: "operator",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	if task.Status != "awaiting_approval" {
		t.Fatalf("task.Status = %q, want awaiting_approval", task.Status)
	}

	select {
	case <-started:
		t.Fatal("task should not start before approval")
	case <-time.After(100 * time.Millisecond):
	}

	approved, err := manager.ApproveTask(task.ID, "launcher", "")
	if err != nil {
		t.Fatalf("ApproveTask() error = %v", err)
	}
	if approved.Status != "queued" {
		t.Fatalf("ApproveTask() status = %q, want queued", approved.Status)
	}

	<-started
	deadline := time.Now().Add(2 * time.Second)
	for {
		current, ok := manager.GetTaskCopy(task.ID)
		if !ok {
			t.Fatalf("task %s disappeared", task.ID)
		}
		if current.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s did not complete after approval, status=%s", task.ID, current.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSubagentManager_RejectTask(t *testing.T) {
	workspace := t.TempDir()
	manager := NewSubagentManager(&MockLLMProvider{}, "test-model", workspace)
	manager.SetTeammateResolver(func(teammateID string) (TaskTeammate, bool) {
		return TaskTeammate{
			ID:             teammateID,
			AgentID:        "main",
			ApprovalPolicy: "advice_only",
		}, true
	})
	manager.SetSpawner(func(
		ctx context.Context,
		task, label, agentID, teammateID string,
		tls *ToolRegistry,
		maxTokens int,
		temperature float64,
		hasMaxTokens, hasTemperature bool,
	) (*ToolResult, error) {
		t.Fatal("spawner should not run for rejected task")
		return nil, nil
	})

	task, err := manager.Spawn(context.Background(), SpawnRequest{
		Task:       "Do not run this",
		TeammateID: "reviewer",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	rejected, err := manager.RejectTask(task.ID, "launcher", "Rejected in review")
	if err != nil {
		t.Fatalf("RejectTask() error = %v", err)
	}
	if rejected.Status != "denied" {
		t.Fatalf("RejectTask() status = %q, want denied", rejected.Status)
	}
	if rejected.RejectedBy != "launcher" {
		t.Fatalf("RejectTask() rejected_by = %q, want launcher", rejected.RejectedBy)
	}
}
