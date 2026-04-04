package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
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
