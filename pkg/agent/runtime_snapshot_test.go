package agent

import (
	"context"
	"errors"
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
	if snapshot.Tasks[0].Cancelable {
		t.Fatal("terminal task should not be cancelable")
	}
}

func TestAgentLoopRuntimeTaskActions(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Tools.Subagent.Enabled = true
	cfg.Tools.Spawn.Enabled = true
	cfg.Tools.SpawnStatus.Enabled = true
	cfg.Agents.List = []config.AgentConfig{
		{ID: "main", Default: true, Name: "Main"},
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
	manager := managerProvider.Manager()

	started := make(chan struct{})
	manager.SetSpawner(func(
		ctx context.Context,
		task, label, targetAgentID, teammateID string,
		tls *tools.ToolRegistry,
		maxTokens int,
		temperature float64,
		hasMaxTokens, hasTemperature bool,
	) (*tools.ToolResult, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})

	task, err := manager.Spawn(context.Background(), tools.SpawnRequest{
		Task:             "Investigate live runtime task actions",
		RequesterAgentID: "main",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	<-started

	info, err := loop.GetRuntimeTask("main", task.ID)
	if err != nil {
		t.Fatalf("GetRuntimeTask() error = %v", err)
	}
	if !info.Cancelable {
		t.Fatal("running task should be cancelable")
	}

	cancelled, err := loop.CancelRuntimeTask("main", task.ID)
	if err != nil {
		t.Fatalf("CancelRuntimeTask() error = %v", err)
	}
	if cancelled.Status != "canceling" {
		t.Fatalf("CancelRuntimeTask() status = %q, want canceling", cancelled.Status)
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
}

func TestAgentLoopRuntimeApprovalAndMemoryReview(t *testing.T) {
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
			ID:             "operator",
			AgentID:        "main",
			MemoryScope:    "teammate:operator",
			ApprovalPolicy: config.ApprovalPolicyConfirmExec,
		},
	}

	loop := NewAgentLoop(cfg, bus.NewMessageBus(), &mockProvider{})
	t.Cleanup(func() {
		loop.GetRegistry().Close()
	})

	agentInst := loop.GetRegistry().GetDefaultAgent()
	rawSpawn, _ := agentInst.Tools.Get("spawn")
	managerProvider := rawSpawn.(runtimeManagerProvider)
	manager := managerProvider.Manager()

	started := make(chan struct{})
	manager.SetSpawner(func(
		ctx context.Context,
		task, label, targetAgentID, teammateID string,
		tls *tools.ToolRegistry,
		maxTokens int,
		temperature float64,
		hasMaxTokens, hasTemperature bool,
	) (*tools.ToolResult, error) {
		close(started)
		return &tools.ToolResult{ForLLM: "Capture the approved server runbook."}, nil
	})

	task, err := manager.Spawn(context.Background(), tools.SpawnRequest{
		Task:       "Restart Home Assistant service",
		TeammateID: "operator",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	info, err := loop.GetRuntimeTask("main", task.ID)
	if err != nil {
		t.Fatalf("GetRuntimeTask() error = %v", err)
	}
	if !info.Approvable || !info.Rejectable {
		t.Fatalf("task approval flags = %#v, want approvable/rejectable", info)
	}

	approved, err := loop.ApproveRuntimeTask("main", task.ID, "launcher", "")
	if err != nil {
		t.Fatalf("ApproveRuntimeTask() error = %v", err)
	}
	if approved.Status != "queued" {
		t.Fatalf("approved.Status = %q, want queued", approved.Status)
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
			t.Fatalf("task %s did not complete, status=%s", task.ID, current.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}

	proposal, err := loop.CreateRuntimeMemoryProposalFromTask("main", task.ID, "shared")
	if err != nil {
		t.Fatalf("CreateRuntimeMemoryProposalFromTask() error = %v", err)
	}
	if proposal.Status != "pending" {
		t.Fatalf("proposal.Status = %q, want pending", proposal.Status)
	}

	updatedProposal, err := loop.UpdateRuntimeMemoryProposal("main", proposal.ID, "operator", MemoryProposalUpdate{
		Scope:   "teammate:operator",
		Title:   "Approved server runbook",
		Content: "Capture the approved server runbook in teammate memory.",
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeMemoryProposal() error = %v", err)
	}
	if updatedProposal.Scope != "teammate:operator" {
		t.Fatalf("updatedProposal.Scope = %q, want teammate:operator", updatedProposal.Scope)
	}
	if updatedProposal.UpdatedBy != "operator" {
		t.Fatalf("updatedProposal.UpdatedBy = %q, want operator", updatedProposal.UpdatedBy)
	}

	approvedProposal, err := loop.ApproveRuntimeMemoryProposal("main", proposal.ID, "launcher", "Ship this")
	if err != nil {
		t.Fatalf("ApproveRuntimeMemoryProposal() error = %v", err)
	}
	if approvedProposal.Status != "approved" {
		t.Fatalf("approvedProposal.Status = %q, want approved", approvedProposal.Status)
	}
	if approvedProposal.ReviewedBy != "launcher" {
		t.Fatalf("approvedProposal.ReviewedBy = %q, want launcher", approvedProposal.ReviewedBy)
	}
	if approvedProposal.ReviewNote != "Ship this" {
		t.Fatalf("approvedProposal.ReviewNote = %q, want Ship this", approvedProposal.ReviewNote)
	}

	snapshot := loop.GetRuntimeSnapshot()
	if snapshot.Summary.MemoryProposalCount != 1 {
		t.Fatalf("MemoryProposalCount = %d, want 1", snapshot.Summary.MemoryProposalCount)
	}
	if snapshot.MemoryProposals[0].Status != "approved" {
		t.Fatalf("snapshot.MemoryProposals[0].Status = %q, want approved", snapshot.MemoryProposals[0].Status)
	}
}

func TestAgentLoopHandoffRuntimeTaskCreatesLinkedChildTask(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Tools.Subagent.Enabled = true
	cfg.Tools.Spawn.Enabled = true
	cfg.Tools.SpawnStatus.Enabled = true
	cfg.Agents.List = []config.AgentConfig{
		{ID: "main", Default: true, Name: "Main"},
	}
	cfg.Teammates.List = []config.TeammateConfig{
		{ID: "coder", AgentID: "main", Role: "coder", MemoryScope: "teammate:coder"},
		{ID: "reviewer", AgentID: "main", Role: "reviewer", MemoryScope: "teammate:reviewer"},
	}

	loop := NewAgentLoop(cfg, bus.NewMessageBus(), &mockProvider{})
	t.Cleanup(func() {
		loop.GetRegistry().Close()
	})

	agentInst := loop.GetRegistry().GetDefaultAgent()
	rawSpawn, _ := agentInst.Tools.Get("spawn")
	managerProvider := rawSpawn.(runtimeManagerProvider)
	manager := managerProvider.Manager()
	manager.SetSpawner(func(
		ctx context.Context,
		task, label, targetAgentID, teammateID string,
		tls *tools.ToolRegistry,
		maxTokens int,
		temperature float64,
		hasMaxTokens, hasTemperature bool,
	) (*tools.ToolResult, error) {
		return &tools.ToolResult{ForLLM: "completed child task"}, nil
	})

	sourceTask, err := manager.Spawn(context.Background(), tools.SpawnRequest{
		Task:       "Implement the feature slice",
		Label:      "feature",
		TeammateID: "coder",
	}, nil)
	if err != nil {
		t.Fatalf("Spawn() source error = %v", err)
	}
	waitForRuntimeTaskTerminalState(t, manager, sourceTask.ID)

	handoffTask, err := loop.HandoffRuntimeTask("main", sourceTask.ID, RuntimeTaskHandoffRequest{
		Actor:      "launcher",
		Note:       "Need a second set of eyes",
		TeammateID: "reviewer",
		Kind:       "review",
	})
	if err != nil {
		t.Fatalf("HandoffRuntimeTask() error = %v", err)
	}
	if handoffTask.Kind != "handoff" {
		t.Fatalf("handoffTask.Kind = %q, want handoff", handoffTask.Kind)
	}
	if handoffTask.ParentTaskID != sourceTask.ID || handoffTask.ParentOwnerAgentID != "main" {
		t.Fatalf("parent link = %s/%s, want main/%s", handoffTask.ParentOwnerAgentID, handoffTask.ParentTaskID, sourceTask.ID)
	}
	if handoffTask.RootTaskID != sourceTask.ID || handoffTask.RootOwnerAgentID != "main" {
		t.Fatalf("root link = %s/%s, want main/%s", handoffTask.RootOwnerAgentID, handoffTask.RootTaskID, sourceTask.ID)
	}
	if handoffTask.HandoffKind != "review" {
		t.Fatalf("HandoffKind = %q, want review", handoffTask.HandoffKind)
	}
	if handoffTask.HandoffActor != "launcher" {
		t.Fatalf("HandoffActor = %q, want launcher", handoffTask.HandoffActor)
	}
	if handoffTask.RequesterTeammateID != "coder" {
		t.Fatalf("RequesterTeammateID = %q, want coder", handoffTask.RequesterTeammateID)
	}

	waitForRuntimeTaskTerminalState(t, manager, handoffTask.ID)

	snapshot := loop.GetRuntimeSnapshot()
	foundChild := false
	for _, task := range snapshot.Tasks {
		if task.ID != handoffTask.ID {
			continue
		}
		foundChild = true
		if !task.Handoffable && task.Status == "completed" {
			t.Fatalf("completed handoff task should be handoffable for follow-up")
		}
	}
	if !foundChild {
		t.Fatalf("handoff task %s missing from runtime snapshot", handoffTask.ID)
	}
}

func waitForRuntimeTaskTerminalState(t *testing.T, manager *tools.SubagentManager, taskID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		task, ok := manager.GetTaskCopy(taskID)
		if !ok {
			t.Fatalf("task %s disappeared", taskID)
		}
		if task.Status == "completed" || task.Status == "failed" || task.Status == "canceled" || task.Status == "denied" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s did not reach terminal state, status=%s", taskID, task.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestUpdateRuntimeMemoryProposal_PreservesValidationErrors(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Tools.Subagent.Enabled = true
	cfg.Tools.Spawn.Enabled = true
	cfg.Tools.SpawnStatus.Enabled = true
	cfg.Agents.List = []config.AgentConfig{
		{ID: "main", Default: true, Name: "Main"},
	}

	loop := NewAgentLoop(cfg, bus.NewMessageBus(), &mockProvider{})
	t.Cleanup(func() {
		loop.GetRegistry().Close()
	})

	proposalStore := runtimeMemoryProposalStoreForAgent(loop.GetRegistry(), "main")
	if proposalStore == nil {
		t.Fatal("expected proposal store for main agent")
	}
	proposal, err := proposalStore.Create(MemoryProposalRequest{
		Scope:   "shared",
		Target:  "long_term",
		Content: "remember this",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = loop.UpdateRuntimeMemoryProposal("main", proposal.ID, "operator", MemoryProposalUpdate{
		Scope:      "shared",
		Domain:     "shared_team",
		EntryType:  "fact",
		Content:    "   ",
		Confidence: "low",
	})
	if !errors.Is(err, ErrRuntimeMemoryProposalInvalid) {
		t.Fatalf("UpdateRuntimeMemoryProposal() error = %v, want ErrRuntimeMemoryProposalInvalid", err)
	}
}
