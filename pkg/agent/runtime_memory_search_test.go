package agent

import (
	"testing"
)

func TestAgentLoop_SearchRuntimeMemoryCatalog(t *testing.T) {
	loop, cfg, _, _, cleanup := newTestAgentLoop(t)
	t.Cleanup(cleanup)
	t.Cleanup(func() { loop.GetRegistry().Close() })

	store := NewMemoryProposalStore(cfg.Agents.Defaults.Workspace)
	proposal, err := store.Create(MemoryProposalRequest{
		Scope:         "teammate:reviewer",
		Domain:        "project",
		Target:        "long_term",
		Kind:          "task_result",
		EntryType:     "decision",
		Title:         "Review Decisions",
		Content:       "Reviewed deployment checklist",
		Confidence:    "high",
		SourceTaskID:  "subagent-42",
		SourceAgentID: "main",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Approve(proposal.ID, "operator", "ship it"); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	catalog := loop.SearchRuntimeMemoryCatalog(RuntimeMemoryCatalogQuery{
		Search:    "deployment",
		Scope:     "teammate:reviewer",
		Domain:    "project",
		EntryType: "decision",
		Archive:   "all",
	})
	if got := len(catalog.Entries); got != 1 {
		t.Fatalf("len(catalog.Entries) = %d, want 1", got)
	}
	entry := catalog.Entries[0]
	if entry.Scope != "teammate:reviewer" {
		t.Fatalf("entry.Scope = %q, want teammate:reviewer", entry.Scope)
	}
	if entry.Domain != "project" {
		t.Fatalf("entry.Domain = %q, want project", entry.Domain)
	}
	if entry.EntryType != "decision" {
		t.Fatalf("entry.EntryType = %q, want decision", entry.EntryType)
	}
	if got := catalog.Summary.EntryCount; got != 1 {
		t.Fatalf("summary.entry_count = %d, want 1", got)
	}
	if got := catalog.Summary.ScopeCount; got != 1 {
		t.Fatalf("summary.scope_count = %d, want 1", got)
	}
	if got := catalog.Summary.DomainCounts["project"]; got != 1 {
		t.Fatalf("summary.domain_counts[project] = %d, want 1", got)
	}
}

func TestAgentLoop_GetRuntimeMemoryHistory(t *testing.T) {
	loop, cfg, _, _, cleanup := newTestAgentLoop(t)
	t.Cleanup(cleanup)
	t.Cleanup(func() { loop.GetRegistry().Close() })

	store := NewMemoryProposalStore(cfg.Agents.Defaults.Workspace)
	approved, err := store.Create(MemoryProposalRequest{
		Scope:               "shared",
		Domain:              "server",
		Target:              "long_term",
		Kind:                "task_result",
		EntryType:           "incident",
		Title:               "Shared Incident",
		Content:             "Captured deployment issue",
		SourceTaskID:        "subagent-1",
		SourceAgentID:       "main",
		RequesterTeammateID: "operator",
	})
	if err != nil {
		t.Fatalf("Create(approved) error = %v", err)
	}
	if _, err := store.Approve(approved.ID, "operator", "approved"); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	pending, err := store.Create(MemoryProposalRequest{
		Scope:         "teammate:reviewer",
		Domain:        "project",
		Target:        "long_term",
		Kind:          "task_result",
		EntryType:     "decision",
		Title:         "Review Follow-up",
		Content:       "Need another pass",
		SourceTaskID:  "subagent-2",
		SourceAgentID: "main",
	})
	if err != nil {
		t.Fatalf("Create(pending) error = %v", err)
	}
	if _, err := store.Update(pending.ID, "launcher", MemoryProposalUpdate{
		Scope:     "teammate:reviewer",
		Domain:    "project",
		EntryType: "decision",
		Title:     "Review Follow-up",
		Content:   "Need another pass",
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	history := loop.GetRuntimeMemoryHistory(RuntimeMemoryHistoryQuery{Limit: 10})
	if len(history.Events) == 0 {
		t.Fatal("expected history events, got none")
	}
	if history.Summary.EventCount != len(history.Events) {
		t.Fatalf("summary.event_count = %d, want %d", history.Summary.EventCount, len(history.Events))
	}
	foundProposalUpdate := false
	foundApproved := false
	for _, event := range history.Events {
		if event.Timestamp == 0 {
			t.Fatalf("event %+v has zero timestamp", event)
		}
		switch event.Kind {
		case "proposal_updated":
			foundProposalUpdate = true
			if event.Actor != "launcher" {
				t.Fatalf("proposal_updated actor = %q, want launcher", event.Actor)
			}
		case "proposal_approved":
			foundApproved = true
		}
	}
	if !foundProposalUpdate {
		t.Fatalf("expected proposal_updated event in %+v", history.Events)
	}
	if !foundApproved {
		t.Fatalf("expected proposal_approved event in %+v", history.Events)
	}
}
