package agent

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestAgentLoop_GetRuntimeMemoryCatalog(t *testing.T) {
	workspace := t.TempDir()

	sharedStore := NewMemoryProposalStore(workspace)
	sharedProposal, err := sharedStore.Create(MemoryProposalRequest{
		Scope:      "shared",
		Domain:     "shared_team",
		Target:     "long_term",
		Kind:       "task_result",
		EntryType:  "decision",
		Title:      "Review workflow",
		Content:    "Approvals should leave an audit trail.",
		Confidence: "high",
	})
	if err != nil {
		t.Fatalf("Create(shared) error = %v", err)
	}
	if _, err := sharedStore.Approve(sharedProposal.ID, "launcher", ""); err != nil {
		t.Fatalf("Approve(shared) error = %v", err)
	}

	teammateStore := NewMemoryProposalStore(workspace)
	teammateProposal, err := teammateStore.Create(MemoryProposalRequest{
		Scope:            "teammate:reviewer",
		Domain:           "teammate_local",
		Target:           "long_term",
		Kind:             "task_result",
		EntryType:        "runbook",
		Title:            "Review checklist",
		Content:          "Check migration, validation, and approval paths before merge.",
		SourceTaskID:     "subagent-9",
		SourceTeammateID: "reviewer",
	})
	if err != nil {
		t.Fatalf("Create(teammate) error = %v", err)
	}
	if _, err := teammateStore.Approve(teammateProposal.ID, "operator", ""); err != nil {
		t.Fatalf("Approve(teammate) error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
		teammates: map[string]TeammateProfile{
			"reviewer": {
				ID:          "reviewer",
				AgentID:     "main",
				MemoryScope: "teammate:reviewer",
			},
		},
	}
	loop := &AgentLoop{registry: registry}

	catalog := loop.GetRuntimeMemoryCatalog()
	if catalog.Summary.WorkspaceCount != 1 {
		t.Fatalf("WorkspaceCount = %d, want 1", catalog.Summary.WorkspaceCount)
	}
	if catalog.Summary.ScopeCount != 2 {
		t.Fatalf("ScopeCount = %d, want 2", catalog.Summary.ScopeCount)
	}
	if catalog.Summary.EntryCount != 2 {
		t.Fatalf("EntryCount = %d, want 2", catalog.Summary.EntryCount)
	}
	if got := catalog.Summary.DomainCounts["shared_team"]; got != 1 {
		t.Fatalf("DomainCounts[shared_team] = %d, want 1", got)
	}
	if got := catalog.Summary.EntryTypeCounts["decision"]; got != 1 {
		t.Fatalf("EntryTypeCounts[decision] = %d, want 1", got)
	}
	if got := catalog.Summary.EntryTypeCounts["runbook"]; got != 1 {
		t.Fatalf("EntryTypeCounts[runbook] = %d, want 1", got)
	}

	foundTeammateEntry := false
	for _, entry := range catalog.Entries {
		if entry.Scope == "teammate:reviewer" {
			foundTeammateEntry = true
			if entry.EntryType != "runbook" {
				t.Fatalf("EntryType = %q, want runbook", entry.EntryType)
			}
			if entry.SourceTaskID != "subagent-9" {
				t.Fatalf("SourceTaskID = %q, want subagent-9", entry.SourceTaskID)
			}
			if entry.SourceTeammateID != "reviewer" {
				t.Fatalf("SourceTeammateID = %q, want reviewer", entry.SourceTeammateID)
			}
		}
	}
	if !foundTeammateEntry {
		t.Fatal("expected teammate memory entry in catalog")
	}
}

func TestAgentLoop_GetRuntimeMemoryCatalog_LegacyMemoryFallback(t *testing.T) {
	workspace := t.TempDir()
	mem := NewMemoryStoreForScope(workspace, "shared")
	if err := mem.WriteLongTerm("Legacy memory that predates reviewed entries."); err != nil {
		t.Fatalf("WriteLongTerm() error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}
	loop := &AgentLoop{registry: registry}

	catalog := loop.GetRuntimeMemoryCatalog()
	if len(catalog.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(catalog.Entries))
	}
	if !catalog.Entries[0].Legacy {
		t.Fatal("expected legacy entry flag to be set")
	}
	if catalog.Entries[0].Domain != "shared_team" {
		t.Fatalf("Domain = %q, want shared_team", catalog.Entries[0].Domain)
	}
}

func TestAgentLoop_ExportRuntimeMemoryCatalog_JSON(t *testing.T) {
	workspace := t.TempDir()
	mem := NewMemoryStoreForScope(workspace, "shared")
	if err := mem.WriteLongTerm("Legacy shared memory"); err != nil {
		t.Fatalf("WriteLongTerm() error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}
	loop := &AgentLoop{registry: registry}

	payload, contentType, filename, err := loop.ExportRuntimeMemoryCatalog("json")
	if err != nil {
		t.Fatalf("ExportRuntimeMemoryCatalog(json) error = %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("contentType = %q, want application/json", contentType)
	}
	if filename != "memory-catalog.json" {
		t.Fatalf("filename = %q, want memory-catalog.json", filename)
	}

	var catalog RuntimeMemoryCatalog
	if err := json.Unmarshal(payload, &catalog); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if catalog.Summary.EntryCount != 1 {
		t.Fatalf("EntryCount = %d, want 1", catalog.Summary.EntryCount)
	}
}

func TestAgentLoop_ExportRuntimeMemoryCatalog_Markdown(t *testing.T) {
	workspace := t.TempDir()
	mem := NewMemoryStoreForScope(workspace, "shared")
	if err := mem.WriteLongTerm("Legacy shared memory"); err != nil {
		t.Fatalf("WriteLongTerm() error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}
	loop := &AgentLoop{registry: registry}

	payload, contentType, filename, err := loop.ExportRuntimeMemoryCatalog("markdown")
	if err != nil {
		t.Fatalf("ExportRuntimeMemoryCatalog(markdown) error = %v", err)
	}
	if !strings.Contains(string(payload), "# PicoClaw Memory Catalog Export") {
		t.Fatalf("export missing title: %s", string(payload))
	}
	if !strings.Contains(string(payload), "Legacy shared memory") {
		t.Fatalf("export missing scope content: %s", string(payload))
	}
	if contentType != "text/markdown; charset=utf-8" {
		t.Fatalf("contentType = %q, want text/markdown; charset=utf-8", contentType)
	}
	if filename != "memory-catalog.md" {
		t.Fatalf("filename = %q, want memory-catalog.md", filename)
	}
}

func TestAgentLoop_ExportRuntimeMemoryCatalog_RejectsUnknownFormat(t *testing.T) {
	loop := &AgentLoop{}
	_, _, _, err := loop.ExportRuntimeMemoryCatalog("csv")
	if !errors.Is(err, ErrRuntimeMemoryCatalogInvalid) {
		t.Fatalf("err = %v, want ErrRuntimeMemoryCatalogInvalid", err)
	}
}
