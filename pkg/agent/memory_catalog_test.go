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

func TestAgentLoop_MemoryCatalogLifecycleActions(t *testing.T) {
	workspace := t.TempDir()
	store := NewMemoryProposalStore(workspace)
	proposal, err := store.Create(MemoryProposalRequest{
		Scope:     "shared",
		Domain:    "shared_team",
		Target:    "long_term",
		Kind:      "task_result",
		EntryType: "fact",
		Title:     "Lifecycle entry",
		Content:   "This entry should be pinnable and archivable.",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Approve(proposal.ID, "launcher", ""); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	loop := &AgentLoop{registry: &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}}
	initial := loop.GetRuntimeMemoryCatalog()
	if len(initial.Entries) != 1 {
		t.Fatalf("len(initial.Entries) = %d, want 1", len(initial.Entries))
	}
	entryID := initial.Entries[0].ID
	if entryID == "" {
		t.Fatal("expected stable entry ID")
	}

	pinned, err := loop.PinRuntimeMemoryCatalogEntry(entryID, "operator")
	if err != nil {
		t.Fatalf("PinRuntimeMemoryCatalogEntry() error = %v", err)
	}
	if !pinned.Pinned || pinned.PinnedBy != "operator" {
		t.Fatalf("pinned lifecycle = %#v", pinned)
	}

	archived, err := loop.ArchiveRuntimeMemoryCatalogEntry(entryID, "operator")
	if err != nil {
		t.Fatalf("ArchiveRuntimeMemoryCatalogEntry() error = %v", err)
	}
	if !archived.Archived || archived.ArchivedBy != "operator" {
		t.Fatalf("archived lifecycle = %#v", archived)
	}

	catalog := loop.GetRuntimeMemoryCatalog()
	if catalog.Summary.PinnedCount != 1 {
		t.Fatalf("PinnedCount = %d, want 1", catalog.Summary.PinnedCount)
	}
	if catalog.Summary.ArchivedCount != 1 {
		t.Fatalf("ArchivedCount = %d, want 1", catalog.Summary.ArchivedCount)
	}
	if len(catalog.Entries) != 1 || catalog.Entries[0].ID != entryID {
		t.Fatalf("catalog entry IDs changed unexpectedly: %#v", catalog.Entries)
	}

	restored, err := loop.RestoreRuntimeMemoryCatalogEntry(entryID, "launcher")
	if err != nil {
		t.Fatalf("RestoreRuntimeMemoryCatalogEntry() error = %v", err)
	}
	if restored.Archived {
		t.Fatalf("expected restored entry to clear archived flag: %#v", restored)
	}
	unpinned, err := loop.UnpinRuntimeMemoryCatalogEntry(entryID, "launcher")
	if err != nil {
		t.Fatalf("UnpinRuntimeMemoryCatalogEntry() error = %v", err)
	}
	if unpinned.Pinned {
		t.Fatalf("expected unpinned entry to clear pinned flag: %#v", unpinned)
	}
}

func TestAgentLoop_MemoryCatalogEntryIDsRemainStableWhenEarlierEntriesShift(t *testing.T) {
	workspace := t.TempDir()
	mem := NewMemoryStoreForScope(workspace, "shared")
	baseContent := strings.Join([]string{
		"## First Entry",
		"",
		"- Added: 2026-04-07 10:00:00 UTC",
		"- Domain: shared_team",
		"",
		"First body",
		"",
		"## Stable Target",
		"",
		"- Added: 2026-04-07 11:00:00 UTC",
		"- Domain: shared_team",
		"",
		"Target body",
	}, "\n")
	if err := mem.WriteLongTerm(baseContent); err != nil {
		t.Fatalf("WriteLongTerm(baseContent) error = %v", err)
	}

	loop := &AgentLoop{registry: &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}}
	before := loop.GetRuntimeMemoryCatalog()
	idsBefore := map[string]string{}
	for _, entry := range before.Entries {
		idsBefore[entry.Title] = entry.ID
	}

	updatedContent := strings.Join([]string{
		"## Prepended Entry",
		"",
		"- Added: 2026-04-07 09:30:00 UTC",
		"- Domain: shared_team",
		"",
		"Prepended body",
		"",
		baseContent,
	}, "\n")
	if err := mem.WriteLongTerm(updatedContent); err != nil {
		t.Fatalf("WriteLongTerm(updatedContent) error = %v", err)
	}

	after := loop.GetRuntimeMemoryCatalog()
	idsAfter := map[string]string{}
	for _, entry := range after.Entries {
		idsAfter[entry.Title] = entry.ID
	}

	if idsBefore["Stable Target"] == "" || idsAfter["Stable Target"] == "" {
		t.Fatalf("missing stable target IDs before=%q after=%q", idsBefore["Stable Target"], idsAfter["Stable Target"])
	}
	if idsBefore["Stable Target"] != idsAfter["Stable Target"] {
		t.Fatalf("stable target ID changed: before=%q after=%q", idsBefore["Stable Target"], idsAfter["Stable Target"])
	}
}

func TestRuntimeMemoryCatalogStateStoreSharedByWorkspace(t *testing.T) {
	workspace := t.TempDir()
	first := getRuntimeMemoryCatalogStateStore(workspace)
	second := getRuntimeMemoryCatalogStateStore(workspace)
	if first != second {
		t.Fatal("expected workspace state store to be shared")
	}

	if err := first.setPinned("memory-a", true, "operator"); err != nil {
		t.Fatalf("setPinned() error = %v", err)
	}
	if err := second.setArchived("memory-b", true, "operator"); err != nil {
		t.Fatalf("setArchived() error = %v", err)
	}
	if _, ok := first.getCopy("memory-b"); !ok {
		t.Fatal("expected first store to observe archive written by second store")
	}
	if _, ok := second.getCopy("memory-a"); !ok {
		t.Fatal("expected second store to observe pin written by first store")
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

func TestAgentLoop_GetRuntimeMemoryCatalog_DoesNotSplitContentHeadings(t *testing.T) {
	workspace := t.TempDir()
	store := NewMemoryProposalStore(workspace)
	proposal, err := store.Create(MemoryProposalRequest{
		Scope:     "shared",
		Domain:    "shared_team",
		Target:    "long_term",
		Kind:      "task_result",
		EntryType: "fact",
		Title:     "Structured notes",
		Content: strings.Join([]string{
			"Keep this as one entry.",
			"",
			"## Summary",
			"",
			"- This heading is part of the body, not a new memory entry.",
		}, "\n"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Approve(proposal.ID, "launcher", ""); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}

	loop := &AgentLoop{registry: &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}}
	catalog := loop.GetRuntimeMemoryCatalog()
	if len(catalog.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(catalog.Entries))
	}
	if !strings.Contains(catalog.Entries[0].Content, "## Summary") {
		t.Fatalf("entry content missing embedded heading: %q", catalog.Entries[0].Content)
	}
}

func TestAgentLoop_GetRuntimeMemoryCatalog_DeduplicatesAliasedScopesByPath(t *testing.T) {
	workspace := t.TempDir()
	aliasScope := "teammate:review:qa"
	mem := NewMemoryStoreForScope(workspace, aliasScope)
	if err := mem.WriteLongTerm(strings.Join([]string{
		"## Review QA Memory",
		"",
		"- Added: 2026-04-06 12:00:00 UTC",
		"- Domain: teammate_local",
		"",
		"One canonical entry",
	}, "\n")); err != nil {
		t.Fatalf("WriteLongTerm() error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
		teammates: map[string]TeammateProfile{
			"review-qa": {
				ID:          "review-qa",
				AgentID:     "main",
				MemoryScope: aliasScope,
			},
		},
	}
	loop := &AgentLoop{registry: registry}
	catalog := loop.GetRuntimeMemoryCatalog()

	if len(catalog.Scopes) != 2 {
		t.Fatalf("len(Scopes) = %d, want 2 (shared + alias)", len(catalog.Scopes))
	}
	if len(catalog.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(catalog.Entries))
	}
	if got := catalog.Summary.EntryCount; got != 1 {
		t.Fatalf("EntryCount = %d, want 1", got)
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
