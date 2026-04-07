package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/fileutil"
)

var ErrRuntimeMemoryBackupInvalid = errors.New("runtime memory backup invalid")
var ErrRuntimeMemoryBackupUnsupportedMode = errors.New("runtime memory backup mode unsupported")

const runtimeMemoryBackupVersion = 1

var runtimeMemoryBackupWriteLongTerm = func(mem *MemoryStore, content string) error {
	return mem.WriteLongTerm(content)
}

var runtimeMemoryBackupWriteFileAtomic = fileutil.WriteFileAtomic

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

type runtimeMemoryBackupFileSnapshot struct {
	path    string
	data    []byte
	exists  bool
	perm    os.FileMode
	dirPerm os.FileMode
}

type runtimeMemoryBackupWorkspaceRestoreState struct {
	memoryRoot       string
	backupRoot       string
	hadExistingRoot  bool
	proposalSnapshot runtimeMemoryBackupFileSnapshot
	catalogSnapshot  runtimeMemoryBackupFileSnapshot
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
	validatedWorkspaces := make([]string, 0, len(backup.Workspaces))
	seenWorkspaces := make(map[string]struct{}, len(backup.Workspaces))
	for _, workspaceBackup := range backup.Workspaces {
		workspace := filepath.Clean(strings.TrimSpace(workspaceBackup.Workspace))
		if workspace == "" || !allowedWorkspaces[workspace] {
			return RuntimeMemoryBackupRestoreResult{}, fmt.Errorf("%w: workspace %q is not part of this runtime", ErrRuntimeMemoryBackupInvalid, workspaceBackup.Workspace)
		}
		if _, ok := seenWorkspaces[workspace]; ok {
			return RuntimeMemoryBackupRestoreResult{}, fmt.Errorf("%w: duplicate workspace %q in backup", ErrRuntimeMemoryBackupInvalid, workspaceBackup.Workspace)
		}
		seenWorkspaces[workspace] = struct{}{}
		if err := validateRuntimeMemoryBackupWorkspace(workspace, workspaceBackup); err != nil {
			return RuntimeMemoryBackupRestoreResult{}, err
		}
		validatedWorkspaces = append(validatedWorkspaces, workspace)
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
	}
	if mode == "replace" {
		appliedStates := make([]runtimeMemoryBackupWorkspaceRestoreState, 0, len(backup.Workspaces))
		for i, workspaceBackup := range backup.Workspaces {
			restoreState, err := restoreRuntimeMemoryBackupWorkspace(validatedWorkspaces[i], workspaceBackup)
			if err != nil {
				for j := len(appliedStates) - 1; j >= 0; j-- {
					if rollbackErr := runtimeMemoryBackupRollbackWorkspaceRestoreState(appliedStates[j]); rollbackErr != nil {
						err = errors.Join(err, rollbackErr)
					}
				}
				return RuntimeMemoryBackupRestoreResult{}, err
			}
			appliedStates = append(appliedStates, restoreState)
		}
		for _, state := range appliedStates {
			if err := runtimeMemoryBackupFinalizeWorkspaceRestore(state); err != nil {
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

func validateRuntimeMemoryBackupWorkspace(workspacePath string, workspace RuntimeMemoryBackupWorkspace) error {
	memoryRoot := filepath.Clean(filepath.Join(workspacePath, "memory"))
	scopeDestinations := make(map[string]string)
	for _, scope := range workspace.Scopes {
		if strings.TrimSpace(scope.Scope) == "" {
			return fmt.Errorf("%w: backup scope is required", ErrRuntimeMemoryBackupInvalid)
		}
		scopePath := filepath.Clean(runtimeMemoryBackupScopeLongTermPath(workspacePath, scope.Scope))
		if !runtimeMemoryBackupPathWithinRoot(memoryRoot, scopePath) {
			return fmt.Errorf("%w: scope %q resolves outside memory root %q", ErrRuntimeMemoryBackupInvalid, scope.Scope, memoryRoot)
		}
		scopeDestinationKey := runtimeMemoryBackupCollisionKey(scopePath)
		if existingScope, ok := scopeDestinations[scopeDestinationKey]; ok {
			return fmt.Errorf("%w: scopes %q and %q resolve to the same memory path %q", ErrRuntimeMemoryBackupInvalid, existingScope, scope.Scope, scopePath)
		}
		scopeDestinations[scopeDestinationKey] = scope.Scope
		seenNotes := make(map[string]struct{}, len(scope.DailyNotes))
		for _, note := range scope.DailyNotes {
			if !runtimeMemoryBackupDailyNotePathOK(note.RelativePath) {
				return fmt.Errorf("%w: invalid daily note path %q", ErrRuntimeMemoryBackupInvalid, note.RelativePath)
			}
			canonicalNotePath := runtimeMemoryBackupCollisionKey(filepath.ToSlash(filepath.Clean(filepath.FromSlash(note.RelativePath))))
			if _, ok := seenNotes[canonicalNotePath]; ok {
				return fmt.Errorf("%w: duplicate daily note path %q in scope %q", ErrRuntimeMemoryBackupInvalid, note.RelativePath, scope.Scope)
			}
			seenNotes[canonicalNotePath] = struct{}{}
		}
	}
	seenProposalIDs := make(map[string]struct{}, len(workspace.Proposals))
	for _, proposal := range workspace.Proposals {
		id := strings.TrimSpace(proposal.ID)
		if id == "" {
			return fmt.Errorf("%w: memory proposal ID is required", ErrRuntimeMemoryBackupInvalid)
		}
		if _, ok := seenProposalIDs[id]; ok {
			return fmt.Errorf("%w: duplicate memory proposal ID %q", ErrRuntimeMemoryBackupInvalid, id)
		}
		seenProposalIDs[id] = struct{}{}
	}
	seenLifecycleIDs := make(map[string]struct{}, len(workspace.LifecycleEntries))
	for _, entry := range workspace.LifecycleEntries {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			return fmt.Errorf("%w: runtime memory catalog entry ID is required", ErrRuntimeMemoryBackupInvalid)
		}
		if _, ok := seenLifecycleIDs[id]; ok {
			return fmt.Errorf("%w: duplicate runtime memory catalog entry ID %q", ErrRuntimeMemoryBackupInvalid, id)
		}
		seenLifecycleIDs[id] = struct{}{}
	}
	return nil
}

func restoreRuntimeMemoryBackupWorkspace(workspace string, backup RuntimeMemoryBackupWorkspace) (runtimeMemoryBackupWorkspaceRestoreState, error) {
	memoryRoot := filepath.Join(workspace, "memory")
	proposalStateFile := filepath.Join(workspace, "state", "memory", "proposals.json")
	catalogStateFile := filepath.Join(workspace, "state", "memory", "catalog_state.json")
	proposalSnapshot, err := runtimeMemoryBackupCaptureFile(proposalStateFile)
	if err != nil {
		return runtimeMemoryBackupWorkspaceRestoreState{}, err
	}
	catalogSnapshot, err := runtimeMemoryBackupCaptureFile(catalogStateFile)
	if err != nil {
		return runtimeMemoryBackupWorkspaceRestoreState{}, err
	}
	stagingRoot, err := os.MkdirTemp(workspace, ".memory-restore-*")
	if err != nil {
		return runtimeMemoryBackupWorkspaceRestoreState{}, err
	}
	defer os.RemoveAll(stagingRoot)

	stagingWorkspace := filepath.Join(stagingRoot, "workspace")
	stagedMemoryRoot := filepath.Join(stagingWorkspace, "memory")
	if err := os.MkdirAll(stagedMemoryRoot, 0o755); err != nil {
		return runtimeMemoryBackupWorkspaceRestoreState{}, err
	}
	for _, scopeBackup := range backup.Scopes {
		mem := NewMemoryStoreForScope(stagingWorkspace, scopeBackup.Scope)
		if !runtimeMemoryBackupPathWithinRoot(stagedMemoryRoot, mem.LongTermPath()) {
			return runtimeMemoryBackupWorkspaceRestoreState{}, fmt.Errorf("%w: scope %q resolves outside memory root %q", ErrRuntimeMemoryBackupInvalid, scopeBackup.Scope, stagedMemoryRoot)
		}
		if err := runtimeMemoryBackupWriteLongTerm(mem, scopeBackup.LongTermContent); err != nil {
			return runtimeMemoryBackupWorkspaceRestoreState{}, err
		}
		for _, note := range scopeBackup.DailyNotes {
			relPath := filepath.FromSlash(note.RelativePath)
			notePath := filepath.Join(mem.memoryDir, relPath)
			if !runtimeMemoryBackupDailyNotePathOK(note.RelativePath) {
				return runtimeMemoryBackupWorkspaceRestoreState{}, fmt.Errorf("%w: invalid daily note path %q", ErrRuntimeMemoryBackupInvalid, note.RelativePath)
			}
			if !runtimeMemoryBackupPathWithinRoot(mem.memoryDir, notePath) {
				return runtimeMemoryBackupWorkspaceRestoreState{}, fmt.Errorf("%w: daily note %q resolves outside scope memory root", ErrRuntimeMemoryBackupInvalid, note.RelativePath)
			}
			if err := os.MkdirAll(filepath.Dir(notePath), 0o755); err != nil {
				return runtimeMemoryBackupWorkspaceRestoreState{}, err
			}
			if err := runtimeMemoryBackupWriteFileAtomic(notePath, []byte(note.Content), 0o600); err != nil {
				return runtimeMemoryBackupWorkspaceRestoreState{}, err
			}
		}
	}
	backupRoot := filepath.Join(workspace, fmt.Sprintf(".memory-backup-%d", time.Now().UnixNano()))
	hadExistingRoot := false
	if _, err := os.Stat(memoryRoot); err == nil {
		hadExistingRoot = true
		if err := os.Rename(memoryRoot, backupRoot); err != nil {
			return runtimeMemoryBackupWorkspaceRestoreState{}, err
		}
	}
	if err := os.Rename(stagedMemoryRoot, memoryRoot); err != nil {
		if hadExistingRoot {
			_ = os.Rename(backupRoot, memoryRoot)
		}
		return runtimeMemoryBackupWorkspaceRestoreState{}, err
	}

	if err := persistRuntimeMemoryProposalBackup(workspace, backup.Proposals); err != nil {
		if rollbackErr := runtimeMemoryBackupRollbackWorkspaceRestore(workspace, memoryRoot, backupRoot, hadExistingRoot, proposalSnapshot, catalogSnapshot); rollbackErr != nil {
			return runtimeMemoryBackupWorkspaceRestoreState{}, errors.Join(err, rollbackErr)
		}
		return runtimeMemoryBackupWorkspaceRestoreState{}, err
	}
	if err := persistRuntimeMemoryCatalogStateBackup(workspace, backup.LifecycleEntries); err != nil {
		if rollbackErr := runtimeMemoryBackupRollbackWorkspaceRestore(workspace, memoryRoot, backupRoot, hadExistingRoot, proposalSnapshot, catalogSnapshot); rollbackErr != nil {
			return runtimeMemoryBackupWorkspaceRestoreState{}, errors.Join(err, rollbackErr)
		}
		return runtimeMemoryBackupWorkspaceRestoreState{}, err
	}
	return runtimeMemoryBackupWorkspaceRestoreState{
		memoryRoot:       memoryRoot,
		backupRoot:       backupRoot,
		hadExistingRoot:  hadExistingRoot,
		proposalSnapshot: proposalSnapshot,
		catalogSnapshot:  catalogSnapshot,
	}, nil
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
	return runtimeMemoryBackupWriteFileAtomic(storeFile, payload, 0o600)
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
	if err := runtimeMemoryBackupWriteFileAtomic(stateFile, payload, 0o600); err != nil {
		return err
	}
	store := getRuntimeMemoryCatalogStateStore(workspace)
	store.replace(entries)
	return nil
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

func runtimeMemoryBackupCaptureFile(path string) (runtimeMemoryBackupFileSnapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return runtimeMemoryBackupFileSnapshot{path: path}, nil
		}
		return runtimeMemoryBackupFileSnapshot{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeMemoryBackupFileSnapshot{}, err
	}
	dirPerm := os.FileMode(0o755)
	if dirInfo, dirErr := os.Stat(filepath.Dir(path)); dirErr == nil {
		dirPerm = dirInfo.Mode().Perm()
	}
	return runtimeMemoryBackupFileSnapshot{
		path:    path,
		data:    data,
		exists:  true,
		perm:    info.Mode().Perm(),
		dirPerm: dirPerm,
	}, nil
}

func runtimeMemoryBackupRestoreFile(snapshot runtimeMemoryBackupFileSnapshot) error {
	if !snapshot.exists {
		if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(snapshot.path), snapshot.dirPerm); err != nil {
		return err
	}
	return runtimeMemoryBackupWriteFileAtomic(snapshot.path, snapshot.data, snapshot.perm)
}

func runtimeMemoryBackupRollbackWorkspaceRestore(
	workspace string,
	memoryRoot, backupRoot string,
	hadExistingRoot bool,
	proposalSnapshot, catalogSnapshot runtimeMemoryBackupFileSnapshot,
) error {
	var errs []string
	if err := os.RemoveAll(memoryRoot); err != nil && !os.IsNotExist(err) {
		errs = append(errs, err.Error())
	}
	if hadExistingRoot {
		if err := os.Rename(backupRoot, memoryRoot); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err.Error())
		}
	}
	if err := runtimeMemoryBackupRestoreFile(proposalSnapshot); err != nil {
		errs = append(errs, err.Error())
	}
	if err := runtimeMemoryBackupRestoreFile(catalogSnapshot); err != nil {
		errs = append(errs, err.Error())
	} else if err := runtimeMemoryBackupRestoreCatalogStateStore(workspace, catalogSnapshot); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("rollback runtime memory backup restore: %s", strings.Join(errs, "; "))
	}
	return nil
}

func runtimeMemoryBackupRollbackWorkspaceRestoreState(state runtimeMemoryBackupWorkspaceRestoreState) error {
	return runtimeMemoryBackupRollbackWorkspaceRestore(
		filepath.Dir(state.memoryRoot),
		state.memoryRoot,
		state.backupRoot,
		state.hadExistingRoot,
		state.proposalSnapshot,
		state.catalogSnapshot,
	)
}

func runtimeMemoryBackupFinalizeWorkspaceRestore(state runtimeMemoryBackupWorkspaceRestoreState) error {
	if !state.hadExistingRoot {
		return nil
	}
	if err := os.RemoveAll(state.backupRoot); err != nil {
		return err
	}
	return nil
}

func runtimeMemoryBackupRestoreCatalogStateStore(workspace string, snapshot runtimeMemoryBackupFileSnapshot) error {
	store := getRuntimeMemoryCatalogStateStore(workspace)
	if !snapshot.exists {
		store.replace(nil)
		return nil
	}
	var payload runtimeMemoryCatalogStateFile
	if err := json.Unmarshal(snapshot.data, &payload); err != nil {
		return err
	}
	if payload.Version != 0 && payload.Version != runtimeMemoryCatalogStateVersion {
		return fmt.Errorf("unsupported memory catalog state version: got %d, want %d", payload.Version, runtimeMemoryCatalogStateVersion)
	}
	store.replace(payload.Entries)
	return nil
}

func runtimeMemoryBackupScopeLongTermPath(workspace, scope string) string {
	return filepath.Join(resolveMemoryDir(workspace, scope), "MEMORY.md")
}

func runtimeMemoryBackupPathWithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func runtimeMemoryBackupCollisionKey(path string) string {
	key := filepath.Clean(path)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		key = strings.ToLower(key)
	}
	return filepath.ToSlash(key)
}
