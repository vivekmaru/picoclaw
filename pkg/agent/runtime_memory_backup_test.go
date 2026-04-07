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

func TestAgentLoop_RestoreRuntimeMemoryBackupDuplicateScopeValidationMatchesFilesystemCollisionKey(t *testing.T) {
	workspace := t.TempDir()
	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main": {ID: "main", Workspace: workspace},
		},
	}
	loop := &AgentLoop{registry: registry}

	scopeA := "team/A"
	scopeB := "team/a"
	pathA := filepath.Clean(runtimeMemoryBackupScopeLongTermPath(workspace, scopeA))
	pathB := filepath.Clean(runtimeMemoryBackupScopeLongTermPath(workspace, scopeB))
	expectDuplicate := runtimeMemoryBackupCollisionKey(pathA) == runtimeMemoryBackupCollisionKey(pathB)

	backup := RuntimeMemoryBackup{
		Version:     runtimeMemoryBackupVersion,
		GeneratedAt: time.Now().UnixMilli(),
		Workspaces: []RuntimeMemoryBackupWorkspace{
			{
				OwnerAgentID: "main",
				Workspace:    workspace,
				Scopes: []RuntimeMemoryBackupScope{
					{Scope: scopeA, LongTermContent: "## Team A\n\nUpper"},
					{Scope: scopeB, LongTermContent: "## Team A\n\nLower"},
				},
			},
		},
	}
	payload, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("Marshal(backup) error = %v", err)
	}

	_, err = loop.RestoreRuntimeMemoryBackup(payload, "validate")
	if expectDuplicate {
		if err == nil || !errors.Is(err, ErrRuntimeMemoryBackupInvalid) {
			t.Fatalf("RestoreRuntimeMemoryBackup(validate) error = %v, want ErrRuntimeMemoryBackupInvalid", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("RestoreRuntimeMemoryBackup(validate) unexpected error = %v", err)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackupRejectsScopesOutsideMemoryRoot(t *testing.T) {
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
						Scope:           "../../outside",
						LongTermContent: "## Escaped\n\nThis should never be restored",
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
		t.Fatalf("escaped scope restore wrote shared memory: %q", got)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackupRejectsDuplicateDailyNotePathsWithinScope(t *testing.T) {
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
						Scope: "shared",
						DailyNotes: []RuntimeMemoryBackupDailyNote{
							{RelativePath: "202604/20260407.md", Content: "first"},
							{RelativePath: "202604/./20260407.md", Content: "second"},
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
}

func TestAgentLoop_RestoreRuntimeMemoryBackupRejectsDuplicateProposalIDs(t *testing.T) {
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
				Proposals: []MemoryProposal{
					{ID: "memory-7", Scope: "shared", Status: "pending", Content: "first"},
					{ID: "memory-7", Scope: "shared", Status: "pending", Content: "second"},
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

func TestAgentLoop_RestoreRuntimeMemoryBackupRejectsDuplicateLifecycleEntryIDs(t *testing.T) {
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
				LifecycleEntries: []runtimeMemoryCatalogEntryState{
					{ID: "memory-1", Pinned: true},
					{ID: "memory-1", Archived: true},
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

func TestAgentLoop_RestoreRuntimeMemoryBackupValidateDoesNotCreateScopeDirectories(t *testing.T) {
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
				Workspace:    "  " + workspace + "  ",
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "team:a",
						LongTermContent: "## Team A\n\nNo writes during validate",
					},
				},
			},
		},
	}
	payload, err := json.Marshal(backup)
	if err != nil {
		t.Fatalf("Marshal(backup) error = %v", err)
	}

	if _, err := loop.RestoreRuntimeMemoryBackup(payload, "validate"); err != nil {
		t.Fatalf("RestoreRuntimeMemoryBackup(validate) error = %v", err)
	}

	scopeDir := filepath.Join(workspace, "memory", "scopes", "team", "a")
	if _, err := os.Stat(scopeDir); !os.IsNotExist(err) {
		t.Fatalf("validate created scope directory %q err=%v", scopeDir, err)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackupRejectsDuplicateWorkspaces(t *testing.T) {
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
					{Scope: "shared", LongTermContent: "## Shared\n\nFirst"},
				},
			},
			{
				OwnerAgentID: "main",
				Workspace:    " " + workspace + " ",
				Scopes: []RuntimeMemoryBackupScope{
					{Scope: "shared", LongTermContent: "## Shared\n\nSecond"},
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

func TestAgentLoop_RestoreRuntimeMemoryBackupRollbackRestoresCatalogCache(t *testing.T) {
	workspaceOne := t.TempDir()
	workspaceTwo := t.TempDir()
	if err := NewMemoryStoreForScope(workspaceOne, "shared").WriteLongTerm("## Shared\n\nWorkspace one original"); err != nil {
		t.Fatalf("WriteLongTerm(workspaceOne) error = %v", err)
	}
	if err := NewMemoryStoreForScope(workspaceTwo, "shared").WriteLongTerm("## Shared\n\nWorkspace two original"); err != nil {
		t.Fatalf("WriteLongTerm(workspaceTwo) error = %v", err)
	}
	workspaceOneState := getRuntimeMemoryCatalogStateStore(workspaceOne)
	workspaceOneState.replace([]runtimeMemoryCatalogEntryState{
		{ID: "workspace-one-original", Pinned: true, PinnedAt: 123, PinnedBy: "launcher"},
	})
	if err := persistRuntimeMemoryCatalogStateBackup(workspaceOne, workspaceOneState.listCopies()); err != nil {
		t.Fatalf("persist workspace one original lifecycle state error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main":   {ID: "main", Workspace: workspaceOne},
			"helper": {ID: "helper", Workspace: workspaceTwo},
		},
	}
	loop := &AgentLoop{registry: registry}

	originalWriteFileAtomic := runtimeMemoryBackupWriteFileAtomic
	runtimeMemoryBackupWriteFileAtomic = func(path string, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(path, filepath.Join("state", "memory", "proposals.json")) && strings.HasPrefix(path, filepath.Clean(workspaceTwo)+string(filepath.Separator)) {
			return errors.New("simulated workspace two proposal write failure")
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
				Workspace:    workspaceOne,
				Scopes: []RuntimeMemoryBackupScope{
					{Scope: "shared", LongTermContent: "## Shared\n\nWorkspace one replacement"},
				},
				LifecycleEntries: []runtimeMemoryCatalogEntryState{
					{ID: "workspace-one-replacement", Archived: true, ArchivedAt: 456, ArchivedBy: "launcher"},
				},
			},
			{
				OwnerAgentID: "helper",
				Workspace:    workspaceTwo,
				Scopes: []RuntimeMemoryBackupScope{
					{Scope: "shared", LongTermContent: "## Shared\n\nWorkspace two replacement"},
				},
				Proposals: []MemoryProposal{
					{ID: "memory-22", Scope: "shared", Status: "pending", Content: "trigger failure"},
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

	stateEntries := getRuntimeMemoryCatalogStateStore(workspaceOne).listCopies()
	if len(stateEntries) != 1 || stateEntries[0].ID != "workspace-one-original" || !stateEntries[0].Pinned {
		t.Fatalf("workspace one catalog cache was not rolled back: %#v", stateEntries)
	}
}

func TestAgentLoop_RestoreRuntimeMemoryBackupReplaceReportsRollbackFailure(t *testing.T) {
	workspace := t.TempDir()
	shared := NewMemoryStoreForScope(workspace, "shared")
	if err := shared.WriteLongTerm("## Shared\n\nOriginal shared memory"); err != nil {
		t.Fatalf("WriteLongTerm(shared) error = %v", err)
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
	callCount := 0
	runtimeMemoryBackupWriteFileAtomic = func(path string, data []byte, perm os.FileMode) error {
		callCount++
		switch filepath.Base(path) {
		case "proposals.json":
			return errors.New("simulated proposal write failure")
		case "catalog_state.json":
			return errors.New("simulated rollback catalog restore failure")
		default:
			return originalWriteFileAtomic(path, data, perm)
		}
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
					{Scope: "shared", LongTermContent: "## Shared\n\nReplacement shared memory"},
				},
				Proposals: []MemoryProposal{
					{
						ID:        "memory-7",
						Scope:     "shared",
						Domain:    "shared_team",
						Target:    "long_term",
						Kind:      "task_result",
						EntryType: "fact",
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

	_, err = loop.RestoreRuntimeMemoryBackup(payload, "replace")
	if err == nil {
		t.Fatal("RestoreRuntimeMemoryBackup(replace) error = nil, want combined failure")
	}
	if !strings.Contains(err.Error(), "simulated proposal write failure") || !strings.Contains(err.Error(), "simulated rollback catalog restore failure") {
		t.Fatalf("combined error = %v, want both original and rollback failure", err)
	}
	if got := shared.ReadLongTerm(); !strings.Contains(got, "Original shared memory") {
		t.Fatalf("rollback-reporting failure replaced existing shared memory: %q", got)
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

func TestAgentLoop_RestoreRuntimeMemoryBackupReplaceRollsBackEarlierWorkspacesOnLaterFailure(t *testing.T) {
	workspaceOne := t.TempDir()
	workspaceTwo := t.TempDir()
	if err := NewMemoryStoreForScope(workspaceOne, "shared").WriteLongTerm("## Shared\n\nWorkspace one original"); err != nil {
		t.Fatalf("WriteLongTerm(workspaceOne) error = %v", err)
	}
	if err := NewMemoryStoreForScope(workspaceTwo, "shared").WriteLongTerm("## Shared\n\nWorkspace two original"); err != nil {
		t.Fatalf("WriteLongTerm(workspaceTwo) error = %v", err)
	}

	registry := &AgentRegistry{
		agents: map[string]*AgentInstance{
			"main":   {ID: "main", Workspace: workspaceOne},
			"helper": {ID: "helper", Workspace: workspaceTwo},
		},
	}
	loop := &AgentLoop{registry: registry}

	originalWriteFileAtomic := runtimeMemoryBackupWriteFileAtomic
	runtimeMemoryBackupWriteFileAtomic = func(path string, data []byte, perm os.FileMode) error {
		if strings.HasSuffix(path, filepath.Join("state", "memory", "proposals.json")) && strings.HasPrefix(path, filepath.Clean(workspaceTwo)+string(filepath.Separator)) {
			return errors.New("simulated workspace two proposal write failure")
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
				Workspace:    workspaceOne,
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "shared",
						LongTermContent: "## Shared\n\nWorkspace one replacement",
					},
				},
				Proposals: []MemoryProposal{
					{
						ID:        "memory-11",
						Scope:     "shared",
						Domain:    "shared_team",
						Target:    "long_term",
						Kind:      "task_result",
						EntryType: "fact",
						Status:    "pending",
						Title:     "Workspace one replacement proposal",
						Content:   "Workspace one replacement proposal",
						Created:   111,
					},
				},
			},
			{
				OwnerAgentID: "helper",
				Workspace:    workspaceTwo,
				Scopes: []RuntimeMemoryBackupScope{
					{
						Scope:           "shared",
						LongTermContent: "## Shared\n\nWorkspace two replacement",
					},
				},
				Proposals: []MemoryProposal{
					{
						ID:        "memory-22",
						Scope:     "shared",
						Domain:    "shared_team",
						Target:    "long_term",
						Kind:      "task_result",
						EntryType: "fact",
						Status:    "pending",
						Title:     "Workspace two replacement proposal",
						Content:   "Workspace two replacement proposal",
						Created:   222,
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

	if got := NewMemoryStoreForScope(workspaceOne, "shared").ReadLongTerm(); !strings.Contains(got, "Workspace one original") {
		t.Fatalf("workspace one was not rolled back: %q", got)
	}
	if got := NewMemoryStoreForScope(workspaceTwo, "shared").ReadLongTerm(); !strings.Contains(got, "Workspace two original") {
		t.Fatalf("workspace two was not rolled back: %q", got)
	}
	if proposals := NewMemoryProposalStore(workspaceOne).ListCopies(); len(proposals) != 0 {
		t.Fatalf("workspace one proposals mutated after later failure: %#v", proposals)
	}
}
