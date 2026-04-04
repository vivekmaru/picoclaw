package agent

import (
	"context"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/tools"
)

type runtimeManagerProvider interface {
	Manager() *tools.SubagentManager
}

func TestAgentLoopGetRuntimeSnapshotIncludesTeammatesAndTasks(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Tools.Subagent.Enabled = true
	cfg.Tools.Spawn.Enabled = true
	cfg.Tools.SpawnStatus.Enabled = true
	cfg.Agents.List = []config.AgentConfig{
		{ID: "main", Default: true, Name: "Main"},
	}
	cfg.Teammates.List = []config.TeammateConfig{
		{
			ID:          "reviewer",
			Name:        "Reviewer",
			Role:        "reviewer",
			AgentID:     "main",
			MemoryScope: "teammate:reviewer",
		},
	}

	loop := NewAgentLoop(cfg, bus.NewMessageBus(), &mockProvider{})
	t.Cleanup(func() {
		loop.GetRegistry().Close()
	})

	agentInst := loop.GetRegistry().GetDefaultAgent()
	if agentInst == nil {
		t.Fatal("expected default agent")
	}

	rawSpawn, ok := agentInst.Tools.Get("spawn")
	if !ok {
		t.Fatal("expected spawn tool")
	}
	managerProvider, ok := rawSpawn.(runtimeManagerProvider)
	if !ok || managerProvider.Manager() == nil {
		t.Fatal("expected spawn tool to expose manager")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager := managerProvider.Manager()
	task, err := manager.Spawn(ctx, tools.SpawnRequest{
		Task:                "Review the latest patch",
		Label:               "review",
		TeammateID:          "reviewer",
		RequesterAgentID:    "main",
		RequesterTeammateID: "main",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		copied, ok := manager.GetTaskCopy(task.ID)
		if !ok {
			t.Fatalf("task %s disappeared", task.ID)
		}
		if copied.Status == "canceled" || copied.Status == "failed" || copied.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s did not reach terminal state, status=%s", task.ID, copied.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	snapshot := loop.GetRuntimeSnapshot()
	if snapshot.Summary.TeammateCount != 1 {
		t.Fatalf("TeammateCount = %d, want 1", snapshot.Summary.TeammateCount)
	}
	if len(snapshot.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(snapshot.Tasks))
	}
	if snapshot.Tasks[0].TeammateID != "reviewer" {
		t.Fatalf("task.TeammateID = %q, want reviewer", snapshot.Tasks[0].TeammateID)
	}
	if snapshot.Tasks[0].OwnerAgentID != "main" {
		t.Fatalf("task.OwnerAgentID = %q, want main", snapshot.Tasks[0].OwnerAgentID)
	}
	if snapshot.Tasks[0].MemoryScope != "teammate:reviewer" {
		t.Fatalf("task.MemoryScope = %q, want teammate:reviewer", snapshot.Tasks[0].MemoryScope)
	}
}
