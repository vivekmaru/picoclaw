package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/fileutil"
)

var ErrRuntimeMemoryBackupInvalid = errors.New("runtime memory backup invalid")
var ErrRuntimeMemoryBackupUnsupportedMode = errors.New("runtime memory backup mode unsupported")

const runtimeMemoryBackupVersion = 1

type RuntimeMemoryBackup struct {
	Version     int                            `json:"version"`
	GeneratedAt int64                          `json:"generated_at"`
	Summary     RuntimeMemoryBackupSummary     `json:"summary"`
	Workspaces  []RuntimeMemoryBackupWorkspace `json:"workspaces"`
}

type RuntimeMemoryBackupSummary struct {
	WorkspaceCount      int `json:"workspace_count"`
	ScopeCount          int `json:"scope_count"`
	LongTermFileCount   int `json:"long_term_file_count"`
	DailyNoteCount      int `json:"daily_note_count"`
	ProposalCount       int `json:"proposal_count"`
	LifecycleEntryCount int `json:"lifecycle_entry_count"`
}

type RuntimeMemoryBackupWorkspace struct {
	OwnerAgentID     string                           `json:"owner_agent_id"`
	Workspace        string                           `json:"workspace"`
	Scopes           []RuntimeMemoryBackupScope       `json:"scopes,omitempty"`
	Proposals        []MemoryProposal                 `json:"proposals,omitempty"`
	LifecycleEntries []runtimeMemoryCatalogEntryState `json:"lifecycle_entries,omitempty"`
}

type RuntimeMemoryBackupScope struct {
	Scope           string                         `json:"scope"`
	DisplayName     string                         `json:"display_name"`
	LongTermContent string                         `json:"long_term_content,omitempty"`
	DailyNotes      []RuntimeMemoryBackupDailyNote `json:"daily_notes,omitempty"`
}

type RuntimeMemoryBackupDailyNote struct {
	RelativePath string `json:"relative_path"`
	Content      string `json:"content"`
}

type RuntimeMemoryBackupRestoreResult struct {
	Mode                string `json:"mode"`
	ValidatedOnly       bool   `json:"validated_only"`
	WorkspaceCount      int    `json:"workspace_count"`
	ScopeCount          int    `json:"scope_count"`
	LongTermFileCount   int    `json:"long_term_file_count"`
	DailyNoteCount      int    `json:"daily_note_count"`
	ProposalCount       int    `json:"proposal_count"`
	LifecycleEntryCount int    `json:"lifecycle_entry_count"`
}

func (al *AgentLoop) ExportRuntimeMemoryBackup(format string) ([]byte, string, string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "json"
	}
	if format != "json" {
		return nil, "", "", fmt.Errorf("%w: unsupported export format %q", ErrRuntimeMemoryBackupInvalid, format)
	}

	backup, err := al.GetRuntimeMemoryBackup()
	if err != nil {
		return nil, "", "", err
	}
	payload, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return nil, "", "", err
	}
	return payload, "application/json", "memory-backup.json", nil
}

func (al *AgentLoop) GetRuntimeMemoryBackup() (RuntimeMemoryBackup, error) {
	backup := RuntimeMemoryBackup{
		Version:     runtimeMemoryBackupVersion,
		GeneratedAt: time.Now().UnixMilli(),
	}
	registry := al.GetRegistry()
	if registry == nil {
		return backup, nil
	}

	workspaceRefs := runtimeMemoryCatalogWorkspaces(registry)
	for _, ref := range workspaceRefs {
		workspaceBackup := RuntimeMemoryBackupWorkspace{
			OwnerAgentID: ref.OwnerAgentID,
			Workspace:    ref.Workspace,
		}
		scopes := runtimeMemoryCatalogScopesForWorkspace(registry, ref)
		for _, scopeInfo := range scopes {
			mem := NewMemoryStoreForScope(ref.Workspace, scopeInfo.Scope)
			scopeBackup, err := runtimeMemoryBackupScopeFromStore(mem)
			if err != nil {
				return RuntimeMemoryBackup{}, err
			}
			scopeBackup.DisplayName = scopeInfo.DisplayName
			workspaceBackup.Scopes = append(workspaceBackup.Scopes, scopeBackup)
			backup.Summary.ScopeCount++
			if strings.TrimSpace(scopeBackup.LongTermContent) != "" {
				backup.Summary.LongTermFileCount++
			}
			backup.Summary.DailyNoteCount += len(scopeBackup.DailyNotes)
		}

		proposalStore := NewMemoryProposalStore(ref.Workspace)
		workspaceBackup.Proposals = proposalStore.ListCopies()
		backup.Summary.ProposalCount += len(workspaceBackup.Proposals)

		stateStore := getRuntimeMemoryCatalogStateStore(ref.Workspace)
		workspaceBackup.LifecycleEntries = stateStore.listCopies()
		backup.Summary.LifecycleEntryCount += len(workspaceBackup.LifecycleEntries)

		slices.SortFunc(workspaceBackup.Scopes, func(a, b RuntimeMemoryBackupScope) int {
			if runtimeMemoryScopeSortKey(a.Scope) != runtimeMemoryScopeSortKey(b.Scope) {
				return strings.Compare(runtimeMemoryScopeSortKey(a.Scope), runtimeMemoryScopeSortKey(b.Scope))
			}
			return strings.Compare(a.Scope, b.Scope)
		})
		slices.SortFunc(workspaceBackup.Proposals, func(a, b MemoryProposal) int {
			if a.Created != b.Created {
				if a.Created < b.Created {
					return -1
				}
				return 1
			}
			return strings.Compare(a.ID, b.ID)
		})
		backup.Workspaces = append(backup.Workspaces, workspaceBackup)
	}

	slices.SortFunc(backup.Workspaces, func(a, b RuntimeMemoryBackupWorkspace) int {
		if a.Workspace != b.Workspace {
			return strings.Compare(a.Workspace, b.Workspace)
		}
		return strings.Compare(a.OwnerAgentID, b.OwnerAgentID)
	})
	backup.Summary.WorkspaceCount = len(backup.Workspaces)
	return backup, nil
}

func (al *AgentLoop) RestoreRuntimeMemoryBackup(payload []byte, mode string) (RuntimeMemoryBackupRestoreResult, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "validate"
	}
	if mode != "validate" && mode != "replace" {
		return RuntimeMemoryBackupRestoreResult{}, fmt.Errorf("%w: %s", ErrRuntimeMemoryBackupUnsupportedMode, mode)
	}

	var backup RuntimeMemoryBackup
	if err := json.Unmarshal(payload, &backup); err != nil {
		return RuntimeMemoryBackupRestoreResult{}, fmt.Errorf("%w: %v", ErrRuntimeMemoryBackupInvalid, err)
	}
	if backup.Version != runtimeMemoryBackupVersion {
		return RuntimeMemoryBackupRestoreResult{}, fmt.Errorf("%w: unsupported version %d", ErrRuntimeMemoryBackupInvalid, backup.Version)
	}

	registry := al.GetRegistry()
	if registry == nil {
		return RuntimeMemoryBackupRestoreResult{}, fmt.Errorf("%w: runtime registry unavailable", ErrRuntimeMemoryBackupInvalid)
	}
	allowedWorkspaces := make(map[string]bool)
	for _, ref := range runtimeMemoryCatalogWorkspaces(registry) {
		allowedWorkspaces[filepath.Clean(ref.Workspace)] = true
	}

	result := RuntimeMemoryBackupRestoreResult{Mode: mode, ValidatedOnly: mode == "validate"}
	for _, workspaceBackup := range backup.Workspaces {
		workspace := filepath.Clean(strings.TrimSpace(workspaceBackup.Workspace))
		if workspace == "" || !allowedWorkspaces[workspace] {
			return RuntimeMemoryBackupRestoreResult{}, fmt.Errorf("%w: workspace %q is not part of this runtime", ErrRuntimeMemoryBackupInvalid, workspaceBackup.Workspace)
		}
		if err := validateRuntimeMemoryBackupWorkspace(workspaceBackup); err != nil {
			return RuntimeMemoryBackupRestoreResult{}, err
		}
		result.WorkspaceCount++
		result.ScopeCount += len(workspaceBackup.Scopes)
		result.ProposalCount += len(workspaceBackup.Proposals)
		result.LifecycleEntryCount += len(workspaceBackup.LifecycleEntries)
		for _, scopeBackup := range workspaceBackup.Scopes {
			if strings.TrimSpace(scopeBackup.LongTermContent) != "" {
				result.LongTermFileCount++
			}
			result.DailyNoteCount += len(scopeBackup.DailyNotes)
		}
		if mode == "replace" {
			if err := restoreRuntimeMemoryBackupWorkspace(workspace, workspaceBackup); err != nil {
				return RuntimeMemoryBackupRestoreResult{}, err
			}
		}
	}
	return result, nil
}

func runtimeMemoryBackupScopeFromStore(mem *MemoryStore) (RuntimeMemoryBackupScope, error) {
	scopeBackup := RuntimeMemoryBackupScope{
		Scope:           mem.Scope(),
		DisplayName:     mem.DisplayName(),
		LongTermContent: mem.ReadLongTerm(),
	}
	err := filepath.WalkDir(mem.memoryDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d == nil || d.IsDir() {
			return walkErr
		}
		if filepath.Clean(path) == filepath.Clean(mem.LongTermPath()) || filepath.Ext(path) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(mem.memoryDir, path)
		if err != nil {
			return err
		}
		if !runtimeMemoryBackupDailyNotePathOK(rel) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		scopeBackup.DailyNotes = append(scopeBackup.DailyNotes, RuntimeMemoryBackupDailyNote{
			RelativePath: filepath.ToSlash(rel),
			Content:      string(content),
		})
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return RuntimeMemoryBackupScope{}, err
	}
	slices.SortFunc(scopeBackup.DailyNotes, func(a, b RuntimeMemoryBackupDailyNote) int {
		return strings.Compare(a.RelativePath, b.RelativePath)
	})
	return scopeBackup, nil
}

func validateRuntimeMemoryBackupWorkspace(workspace RuntimeMemoryBackupWorkspace) error {
	for _, scope := range workspace.Scopes {
		if strings.TrimSpace(scope.Scope) == "" {
			return fmt.Errorf("%w: backup scope is required", ErrRuntimeMemoryBackupInvalid)
		}
		for _, note := range scope.DailyNotes {
			if !runtimeMemoryBackupRelativePathOK(note.RelativePath) {
				return fmt.Errorf("%w: invalid daily note path %q", ErrRuntimeMemoryBackupInvalid, note.RelativePath)
			}
		}
	}
	return nil
}

func restoreRuntimeMemoryBackupWorkspace(workspace string, backup RuntimeMemoryBackupWorkspace) error {
	memoryRoot := filepath.Join(workspace, "memory")
	if err := os.RemoveAll(memoryRoot); err != nil {
		return err
	}
	for _, scopeBackup := range backup.Scopes {
		mem := NewMemoryStoreForScope(workspace, scopeBackup.Scope)
		if err := mem.WriteLongTerm(scopeBackup.LongTermContent); err != nil {
			return err
		}
		for _, note := range scopeBackup.DailyNotes {
			relPath := filepath.FromSlash(note.RelativePath)
			notePath := filepath.Join(mem.memoryDir, relPath)
			if !runtimeMemoryBackupRelativePathOK(note.RelativePath) {
				return fmt.Errorf("%w: invalid daily note path %q", ErrRuntimeMemoryBackupInvalid, note.RelativePath)
			}
			if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
				return err
			}
			if err := fileutil.WriteFileAtomic(notePath, []byte(note.Content), 0o600); err != nil {
				return err
			}
		}
	}

	if err := persistRuntimeMemoryProposalBackup(workspace, backup.Proposals); err != nil {
		return err
	}
	if err := persistRuntimeMemoryCatalogStateBackup(workspace, backup.LifecycleEntries); err != nil {
		return err
	}
	return nil
}

func persistRuntimeMemoryProposalBackup(workspace string, proposals []MemoryProposal) error {
	storeFile := filepath.Join(workspace, "state", "memory", "proposals.json")
	nextID := 1
	for _, proposal := range proposals {
		if parsed := parseMemoryProposalNumericID(proposal.ID); parsed >= nextID {
			nextID = parsed + 1
		}
	}
	payload, err := json.MarshalIndent(memoryProposalStoreFile{
		Version:   memoryProposalStoreVersion,
		NextID:    nextID,
		Proposals: proposals,
	}, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(storeFile, payload, 0o600)
}

func persistRuntimeMemoryCatalogStateBackup(workspace string, entries []runtimeMemoryCatalogEntryState) error {
	stateFile := filepath.Join(workspace, "state", "memory", "catalog_state.json")
	payload, err := json.MarshalIndent(runtimeMemoryCatalogStateFile{
		Version: runtimeMemoryCatalogStateVersion,
		Entries: entries,
	}, "", "  ")
	if err != nil {
		return err
	}
	store := getRuntimeMemoryCatalogStateStore(workspace)
	store.replace(entries)
	return fileutil.WriteFileAtomic(stateFile, payload, 0o600)
}

func runtimeMemoryBackupRelativePathOK(path string) bool {
	path = filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
	if path == "." || path == "" || filepath.IsAbs(path) {
		return false
	}
	return !strings.HasPrefix(path, ".."+string(filepath.Separator)) && path != ".."
}

func runtimeMemoryBackupDailyNotePathOK(path string) bool {
	if !runtimeMemoryBackupRelativePathOK(path) {
		return false
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))), "/")
	if len(parts) != 2 {
		return false
	}
	monthDir := parts[0]
	fileName := parts[1]
	if len(monthDir) != 6 || !allDigits(monthDir) {
		return false
	}
	if !strings.HasSuffix(fileName, ".md") {
		return false
	}
	datePart := strings.TrimSuffix(fileName, ".md")
	return len(datePart) == 8 && allDigits(datePart)
}

func allDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}
