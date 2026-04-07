package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAgentLoop_GetRuntimeMemoryBackup(t *testing.T) {
	workspace := t.TempDir()
	shared := NewMemoryStoreForScope(workspace, "shared")
	if err := shared.WriteLongTerm("## Shared\n\nShared memory body"); err != nil {
		t.Fatalf("WriteLongTerm(shared) error = %v", err)
	}
	sharedDaily := shared.dailyFileFor(time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC))
	if err := os.MkdirAll(filepath.Dir(sharedDaily), 0o755); err != nil {
		t.Fatalf("MkdirAll(sharedDaily) error = %v", err)
	}
	if err := os.WriteFile(sharedDaily, []byte("# 2026-04-07\n\nShared note"), 0o600); err != nil {
		t.Fatalf("WriteFile(sharedDaily) error = %v", err)
	}

	teammate := NewMemoryStoreForScope(workspace, "teammate:reviewer")
	if err := teammate.WriteLongTerm("## Reviewer\n\nReviewer memory body"); err != nil {
		t.Fatalf("WriteLongTerm(teammate) error = %v", err)
	}

	proposalStore := NewMemoryProposalStore(workspace)
	if _, err := proposalStore.Create(MemoryProposalRequest{
		Scope:     "shared",
		Domain:    "shared_team",
		Target:    "long_term",
		Kind:      "task_result",
		EntryType: "decision",
		Title:     "Pending decision",
		Content:   "Keep pending proposal in backup",
	}); err != nil {
		t.Fatalf("Create(proposal) error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
		teammates: map[string]TeammateProfile{
			"reviewer": {ID: "reviewer", AgentID: "main", MemoryScope: "teammate:reviewer"},
		},
	}
	loop := &AgentLoop{registry: registry}

	catalog := loop.GetRuntimeMemoryCatalog()
	if len(catalog.Entries) == 0 {
		t.Fatal("expected memory catalog entries")
	}
	if _, err := loop.PinRuntimeMemoryCatalogEntry(catalog.Entries[0].ID, "launcher"); err != nil {
		t.Fatalf("PinRuntimeMemoryCatalogEntry() error = %v", err)
	}

	backup, err := loop.GetRuntimeMemoryBackup()
	if err != nil {
		t.Fatalf("GetRuntimeMemoryBackup() error = %v", err)
	}
	if backup.Version != runtimeMemoryBackupVersion {
		t.Fatalf("Version = %d, want %d", backup.Version, runtimeMemoryBackupVersion)
	}
	if backup.Summary.WorkspaceCount != 1 {
		t.Fatalf("WorkspaceCount = %d, want 1", backup.Summary.WorkspaceCount)
	}
	if backup.Summary.ScopeCount != 2 {
		t.Fatalf("ScopeCount = %d, want 2", backup.Summary.ScopeCount)
	}
	if backup.Summary.DailyNoteCount != 1 {
		t.Fatalf("DailyNoteCount = %d, want 1", backup.Summary.DailyNoteCount)
	}
	if backup.Summary.ProposalCount != 1 {
		t.Fatalf("ProposalCount = %d, want 1", backup.Summary.ProposalCount)
	}
	if backup.Summary.LifecycleEntryCount != 1 {
		t.Fatalf("LifecycleEntryCount = %d, want 1", backup.Summary.LifecycleEntryCount)
	}
	if len(backup.Workspaces) != 1 {
		t.Fatalf("len(Workspaces) = %d, want 1", len(backup.Workspaces))
	}
	if len(backup.Workspaces[0].Scopes) != 2 {
		t.Fatalf("len(Scopes) = %d, want 2", len(backup.Workspaces[0].Scopes))
	}
	if got := backup.Workspaces[0].Scopes[0].LongTermContent + backup.Workspaces[0].Scopes[1].LongTermContent; !strings.Contains(got, "Shared memory body") || !strings.Contains(got, "Reviewer memory body") {
		t.Fatalf("backup missing long-term content: %#v", backup.Workspaces[0].Scopes)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackup(t *testing.T) {
	workspace := t.TempDir()
	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
		teammates: map[string]TeammateProfile{
			"reviewer": {ID: "reviewer", AgentID: "main", MemoryScope: "teammate:reviewer"},
		},
	}
	loop := &AgentLoop{registry: registry}

	backup := RuntimeMemoryBackup{
		Version:     runtimeMemoryBackupVersion,
		GeneratedAt: time.Now().UnixMilli(),
		Workspaces: []RuntimeMemoryBackupWorkspace{
			{
				OwnerAgentID: "main",
				Workspace:    workspace,
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "shared",
						DisplayName:     "Shared Memory",
						LongTermContent: "## Shared\n\nRestored shared memory",
						DailyNotes: []RuntimeMemoryBackupDailyNote{
							{RelativePath: "202604/20260407.md", Content: "# 2026-04-07\n\nRestored shared note"},
						},
					},
					{
						Scope:           "teammate:reviewer",
						DisplayName:     "Teammate Memory (reviewer)",
						LongTermContent: "## Reviewer\n\nRestored reviewer memory",
					},
				},
				Proposals: []MemoryProposal{
					{
						ID:        "memory-7",
						Scope:     "shared",
						Domain:    "shared_team",
						Target:    "long_term",
						Kind:      "task_result",
						EntryType: "decision",
						Status:    "pending",
						Title:     "Restore proposal",
						Content:   "Pending proposal restored from backup",
						Created:   123,
					},
				},
				LifecycleEntries: []runtimeMemoryCatalogEntryState{
					{ID: "memory-restored", Pinned: true, PinnedBy: "launcher", PinnedAt: 123},
				},
			},
		},
	}
	payload, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("Marshal(backup) error = %v", err)
	}

	validateResult, err := loop.RestoreRuntimeMemoryBackup(payload, "validate")
	if err != nil {
		t.Fatalf("RestoreRuntimeMemoryBackup(validate) error = %v", err)
	}
	if !validateResult.ValidatedOnly {
		t.Fatalf("ValidatedOnly = %v, want true", validateResult.ValidatedOnly)
	}
	if shared := NewMemoryStoreForScope(workspace, "shared").ReadLongTerm(); shared != "" {
		t.Fatalf("validate mode wrote shared memory: %q", shared)
	}

	restoreResult, err := loop.RestoreRuntimeMemoryBackup(payload, "replace")
	if err != nil {
		t.Fatalf("RestoreRuntimeMemoryBackup(replace) error = %v", err)
	}
	if restoreResult.ValidatedOnly {
		t.Fatalf("ValidatedOnly = %v, want false", restoreResult.ValidatedOnly)
	}
	if got := NewMemoryStoreForScope(workspace, "shared").ReadLongTerm(); !strings.Contains(got, "Restored shared memory") {
		t.Fatalf("shared long-term = %q", got)
	}
	if got := NewMemoryStoreForScope(workspace, "teammate:reviewer").ReadLongTerm(); !strings.Contains(got, "Restored reviewer memory") {
		t.Fatalf("reviewer long-term = %q", got)
	}
	dailyNote := filepath.Join(workspace, "memory", "202604", "20260407.md")
	if data, err := os.ReadFile(dailyNote); err != nil || !strings.Contains(string(data), "Restored shared note") {
		t.Fatalf("daily note restore = %q err=%v", string(data), err)
	}
	proposals := NewMemoryProposalStore(workspace).ListCopies()
	if len(proposals) != 1 || proposals[0].ID != "memory-7" {
		t.Fatalf("restored proposals = %#v", proposals)
	}
	stateEntries := getRuntimeMemoryCatalogStateStore(workspace).listCopies()
	if len(stateEntries) != 1 || stateEntries[0].ID != "memory-restored" || !stateEntries[0].Pinned {
		t.Fatalf("restored lifecycle entries = %#v", stateEntries)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackupRejectsNonDailyNotePaths(t *testing.T) {
	workspace := t.TempDir()
	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}
	loop := &AgentLoop{registry: registry}

	backup := RuntimeMemoryBackup{
		Version:     runtimeMemoryBackupVersion,
		GeneratedAt: time.Now().UnixMilli(),
		Workspaces: []RuntimeMemoryBackupWorkspace{
			{
				OwnerAgentID: "main",
				Workspace:    workspace,
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "shared",
						LongTermContent: "## Shared\n\nExisting shared memory",
						DailyNotes: []RuntimeMemoryBackupDailyNote{
							{RelativePath: "MEMORY.md", Content: "this must never restore as a daily note"},
						},
					},
				},
			},
		},
	}
	payload, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("Marshal(backup) error = %v", err)
	}

	if _, err := loop.RestoreRuntimeMemoryBackup(payload, "validate"); err == nil || !errors.Is(err, ErrRuntimeMemoryBackupInvalid) {
		t.Fatalf("RestoreRuntimeMemoryBackup(validate) error = %v, want ErrRuntimeMemoryBackupInvalid", err)
	}
	if _, err := loop.RestoreRuntimeMemoryBackup(payload, "replace"); err == nil || !errors.Is(err, ErrRuntimeMemoryBackupInvalid) {
		t.Fatalf("RestoreRuntimeMemoryBackup(replace) error = %v, want ErrRuntimeMemoryBackupInvalid", err)
	}
	if got := NewMemoryStoreForScope(workspace, "shared").ReadLongTerm(); got != "" {
		t.Fatalf("invalid restore wrote shared long-term memory: %q", got)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackupRejectsDuplicateCanonicalScopePaths(t *testing.T) {
	workspace := t.TempDir()
	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}
	loop := &AgentLoop{registry: registry}

	backup := RuntimeMemoryBackup{
		Version:     runtimeMemoryBackupVersion,
		GeneratedAt: time.Now().UnixMilli(),
		Workspaces: []RuntimeMemoryBackupWorkspace{
			{
				OwnerAgentID: "main",
				Workspace:    workspace,
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "team/a",
						LongTermContent: "## Team A\n\nSlash form",
					},
					{
						Scope:           "team:a",
						LongTermContent: "## Team A\n\nColon form",
					},
				},
			},
		},
	}
	payload, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("Marshal(backup) error = %v", err)
	}

	if _, err := loop.RestoreRuntimeMemoryBackup(payload, "validate"); err == nil || !errors.Is(err, ErrRuntimeMemoryBackupInvalid) {
		t.Fatalf("RestoreRuntimeMemoryBackup(validate) error = %v, want ErrRuntimeMemoryBackupInvalid", err)
	}
}

func TestPersistRuntimeMemoryCatalogStateBackupPreservesCachedStateOnWriteFailure(t *testing.T) {
	workspace := t.TempDir()
	store := getRuntimeMemoryCatalogStateStore(workspace)
	store.replace([]runtimeMemoryCatalogEntryState{
		{ID: "existing-entry", Pinned: true, PinnedAt: 123, PinnedBy: "launcher"},
	})

	stateFile := filepath.Join(workspace, "state", "memory", "catalog_state.json")
	if err := os.MkdirAll(stateFile, 0o755); err != nil {
		t.Fatalf("MkdirAll(stateFile-as-dir) error = %v", err)
	}

	err := persistRuntimeMemoryCatalogStateBackup(workspace, []runtimeMemoryCatalogEntryState{
		{ID: "replacement-entry", Archived: true, ArchivedAt: 456, ArchivedBy: "launcher"},
	})
	if err == nil {
		t.Fatal("persistRuntimeMemoryCatalogStateBackup() error = nil, want failure")
	}

	stateEntries := store.listCopies()
	if len(stateEntries) != 1 || stateEntries[0].ID != "existing-entry" || !stateEntries[0].Pinned {
		t.Fatalf("cached lifecycle state mutated after write failure: %#v", stateEntries)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackupReplacePreservesExistingMemoryOnWriteFailure(t *testing.T) {
	workspace := t.TempDir()
	shared := NewMemoryStoreForScope(workspace, "shared")
	if err := shared.WriteLongTerm("## Shared\n\nOriginal shared memory"); err != nil {
		t.Fatalf("WriteLongTerm(shared) error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}
	loop := &AgentLoop{registry: registry}

	originalWriteLongTerm := runtimeMemoryBackupWriteLongTerm
	runtimeMemoryBackupWriteLongTerm = func(mem *MemoryStore, content string) error {
		return errors.New("simulated staged write failure")
	}
	defer func() {
		runtimeMemoryBackupWriteLongTerm = originalWriteLongTerm
	}()

	backup := RuntimeMemoryBackup{
		Version:     runtimeMemoryBackupVersion,
		GeneratedAt: time.Now().UnixMilli(),
		Workspaces: []RuntimeMemoryBackupWorkspace{
			{
				OwnerAgentID: "main",
				Workspace:    workspace,
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "shared",
						LongTermContent: "## Shared\n\nReplacement shared memory",
					},
				},
			},
		},
	}
	payload, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("Marshal(backup) error = %v", err)
	}

	if _, err := loop.RestoreRuntimeMemoryBackup(payload, "replace"); err == nil {
		t.Fatal("RestoreRuntimeMemoryBackup(replace) error = nil, want failure")
	}
	if got := shared.ReadLongTerm(); !strings.Contains(got, "Original shared memory") {
		t.Fatalf("write failure replaced existing shared memory: %q", got)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackupReplaceRollsBackStateWhenLaterPersistFails(t *testing.T) {
	workspace := t.TempDir()
	shared := NewMemoryStoreForScope(workspace, "shared")
	if err := shared.WriteLongTerm("## Shared\n\nOriginal shared memory"); err != nil {
		t.Fatalf("WriteLongTerm(shared) error = %v", err)
	}

	proposalStore := NewMemoryProposalStore(workspace)
	if _, err := proposalStore.Create(MemoryProposalRequest{
		Scope:     "shared",
		Domain:    "shared_team",
		Target:    "long_term",
		Kind:      "task_result",
		EntryType: "fact",
		Title:     "Original proposal",
		Content:   "Original pending proposal",
	}); err != nil {
		t.Fatalf("Create(original proposal) error = %v", err)
	}

	stateStore := getRuntimeMemoryCatalogStateStore(workspace)
	stateStore.replace([]runtimeMemoryCatalogEntryState{
		{ID: "existing-entry", Pinned: true, PinnedAt: 123, PinnedBy: "launcher"},
	})
	if err := persistRuntimeMemoryCatalogStateBackup(workspace, stateStore.listCopies()); err != nil {
		t.Fatalf("persist original lifecycle state error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}
	loop := &AgentLoop{registry: registry}

	originalWriteFileAtomic := runtimeMemoryBackupWriteFileAtomic
	var writeCount int
	runtimeMemoryBackupWriteFileAtomic = func(path string, data []byte, perm os.FileMode) error {
		writeCount++
		if filepath.Base(path) == "catalog_state.json" && writeCount >= 2 {
			return errors.New("simulated catalog state write failure")
		}
		return originalWriteFileAtomic(path, data, perm)
	}
	defer func() {
		runtimeMemoryBackupWriteFileAtomic = originalWriteFileAtomic
	}()

	backup := RuntimeMemoryBackup{
		Version:     runtimeMemoryBackupVersion,
		GeneratedAt: time.Now().UnixMilli(),
		Workspaces: []RuntimeMemoryBackupWorkspace{
			{
				OwnerAgentID: "main",
				Workspace:    workspace,
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "shared",
						LongTermContent: "## Shared\n\nReplacement shared memory",
					},
				},
				Proposals: []MemoryProposal{
					{
						ID:        "memory-7",
						Scope:     "shared",
						Domain:    "shared_team",
						Target:    "long_term",
						Kind:      "task_result",
						EntryType: "decision",
						Status:    "pending",
						Title:     "Replacement proposal",
						Content:   "Replacement proposal content",
						Created:   456,
					},
				},
				LifecycleEntries: []runtimeMemoryCatalogEntryState{
					{ID: "replacement-entry", Archived: true, ArchivedAt: 456, ArchivedBy: "launcher"},
				},
			},
		},
	}
	payload, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("Marshal(backup) error = %v", err)
	}

	if _, err := loop.RestoreRuntimeMemoryBackup(payload, "replace"); err == nil {
		t.Fatal("RestoreRuntimeMemoryBackup(replace) error = nil, want failure")
	}

	if got := shared.ReadLongTerm(); !strings.Contains(got, "Original shared memory") {
		t.Fatalf("later persist failure replaced existing shared memory: %q", got)
	}
	proposals := NewMemoryProposalStore(workspace).ListCopies()
	if len(proposals) != 1 || proposals[0].Title != "Original proposal" {
		t.Fatalf("later persist failure mutated proposals: %#v", proposals)
	}
	stateEntries := getRuntimeMemoryCatalogStateStore(workspace).listCopies()
	if len(stateEntries) != 1 || stateEntries[0].ID != "existing-entry" || !stateEntries[0].Pinned {
		t.Fatalf("later persist failure mutated lifecycle entries: %#v", stateEntries)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackupReplaceValidatesAllWorkspacesBeforeWriting(t *testing.T) {
	workspace := t.TempDir()
	if err := NewMemoryStoreForScope(workspace, "shared").WriteLongTerm("## Shared\n\nOriginal shared memory"); err != nil {
		t.Fatalf("WriteLongTerm(shared) error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}
	loop := &AgentLoop{registry: registry}

	backup := RuntimeMemoryBackup{
		Version:     runtimeMemoryBackupVersion,
		GeneratedAt: time.Now().UnixMilli(),
		Workspaces: []RuntimeMemoryBackupWorkspace{
			{
				OwnerAgentID: "main",
				Workspace:    workspace,
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "shared",
						LongTermContent: "## Shared\n\nReplacement shared memory",
					},
				},
			},
			{
				OwnerAgentID: "missing",
				Workspace:    filepath.Join(workspace, "missing-workspace"),
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "shared",
						LongTermContent: "## Shared\n\nShould never be applied",
					},
				},
			},
		},
	}
	payload, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("Marshal(backup) error = %v", err)
	}

	if _, err := loop.RestoreRuntimeMemoryBackup(payload, "replace"); err == nil || !errors.Is(err, ErrRuntimeMemoryBackupInvalid) {
		t.Fatalf("RestoreRuntimeMemoryBackup(replace) error = %v, want ErrRuntimeMemoryBackupInvalid", err)
	}

	if got := NewMemoryStoreForScope(workspace, "shared").ReadLongTerm(); !strings.Contains(got, "Original shared memory") {
		t.Fatalf("replace mutated validated workspace before failing later validation: %q", got)
	}
}
