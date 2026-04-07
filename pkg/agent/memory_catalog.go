package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var ErrRuntimeMemoryCatalogInvalid = errors.New("runtime memory catalog invalid")
var ErrRuntimeMemoryCatalogEntryNotFound = errors.New("runtime memory catalog entry not found")

type RuntimeMemoryCatalog struct {
	GeneratedAt int64                     `json:"generated_at"`
	Summary     RuntimeMemoryCatalogStats `json:"summary"`
	Scopes      []RuntimeMemoryScopeInfo  `json:"scopes,omitempty"`
	Entries     []RuntimeMemoryEntryInfo  `json:"entries,omitempty"`
}

type RuntimeMemoryCatalogStats struct {
	ScopeCount       int            `json:"scope_count"`
	EntryCount       int            `json:"entry_count"`
	PinnedCount      int            `json:"pinned_count"`
	ArchivedCount    int            `json:"archived_count"`
	DomainCounts     map[string]int `json:"domain_counts,omitempty"`
	EntryTypeCounts  map[string]int `json:"entry_type_counts,omitempty"`
	WorkspaceCount   int            `json:"workspace_count"`
	WorkspaceEntries map[string]int `json:"workspace_entries,omitempty"`
}

type RuntimeMemoryScopeInfo struct {
	OwnerAgentID string `json:"owner_agent_id"`
	Workspace    string `json:"workspace"`
	Scope        string `json:"scope"`
	DisplayName  string `json:"display_name"`
	LongTermPath string `json:"long_term_path"`
	EntryCount   int    `json:"entry_count"`
	HasLongTerm  bool   `json:"has_long_term"`
}

type RuntimeMemoryEntryInfo struct {
	ID               string `json:"id"`
	OwnerAgentID     string `json:"owner_agent_id"`
	Workspace        string `json:"workspace"`
	Scope            string `json:"scope"`
	ScopeDisplayName string `json:"scope_display_name"`
	SourcePath       string `json:"source_path"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	Domain           string `json:"domain,omitempty"`
	EntryType        string `json:"entry_type,omitempty"`
	Confidence       string `json:"confidence,omitempty"`
	AddedAt          int64  `json:"added_at,omitempty"`
	AddedAtDisplay   string `json:"added_at_display,omitempty"`
	SourceTaskID     string `json:"source_task_id,omitempty"`
	SourceTeammateID string `json:"source_teammate_id,omitempty"`
	ReviewedBy       string `json:"reviewed_by,omitempty"`
	Pinned           bool   `json:"pinned,omitempty"`
	PinnedAt         int64  `json:"pinned_at,omitempty"`
	PinnedBy         string `json:"pinned_by,omitempty"`
	Archived         bool   `json:"archived,omitempty"`
	ArchivedAt       int64  `json:"archived_at,omitempty"`
	ArchivedBy       string `json:"archived_by,omitempty"`
	Legacy           bool   `json:"legacy,omitempty"`
}

type runtimeMemoryWorkspaceRef struct {
	OwnerAgentID string
	Workspace    string
}

type runtimeMemoryParsedSection struct {
	Title          string
	Content        string
	AddedAt        int64
	AddedAtDisplay string
	Domain         string
	EntryType      string
	Confidence     string
	SourceTaskID   string
	SourceTeammate string
	ReviewedBy     string
	Legacy         bool
}

func (al *AgentLoop) GetRuntimeMemoryCatalog() RuntimeMemoryCatalog {
	catalog := RuntimeMemoryCatalog{
		GeneratedAt: time.Now().UnixMilli(),
		Summary: RuntimeMemoryCatalogStats{
			DomainCounts:     map[string]int{},
			EntryTypeCounts:  map[string]int{},
			WorkspaceEntries: map[string]int{},
		},
	}

	registry := al.GetRegistry()
	if registry == nil {
		return catalog
	}

	workspaceRefs := runtimeMemoryCatalogWorkspaces(registry)
	catalog.Summary.WorkspaceCount = len(workspaceRefs)

	for _, ref := range workspaceRefs {
		scopes := runtimeMemoryCatalogScopesForWorkspace(registry, ref)
		for _, scopeInfo := range scopes {
			mem := NewMemoryStoreForScope(ref.Workspace, scopeInfo.Scope)
			content := strings.TrimSpace(mem.ReadLongTerm())
			scopeInfo.HasLongTerm = content != ""
			if content != "" {
				stateStore := getRuntimeMemoryCatalogStateStore(ref.Workspace)
				entries := parseRuntimeMemoryCatalogEntries(ref.OwnerAgentID, ref.Workspace, mem, content)
				for i := range entries {
					stateStore.apply(&entries[i])
				}
				scopeInfo.EntryCount = len(entries)
				catalog.Entries = append(catalog.Entries, entries...)
				catalog.Summary.WorkspaceEntries[ref.Workspace] += len(entries)
				for _, entry := range entries {
					if domain := strings.TrimSpace(entry.Domain); domain != "" {
						catalog.Summary.DomainCounts[domain]++
					}
					if entryType := strings.TrimSpace(entry.EntryType); entryType != "" {
						catalog.Summary.EntryTypeCounts[entryType]++
					}
					if entry.Pinned {
						catalog.Summary.PinnedCount++
					}
					if entry.Archived {
						catalog.Summary.ArchivedCount++
					}
				}
			}
			catalog.Scopes = append(catalog.Scopes, scopeInfo)
		}
	}

	slices.SortFunc(catalog.Scopes, func(a, b RuntimeMemoryScopeInfo) int {
		if a.Workspace != b.Workspace {
			return strings.Compare(a.Workspace, b.Workspace)
		}
		if a.OwnerAgentID != b.OwnerAgentID {
			return strings.Compare(a.OwnerAgentID, b.OwnerAgentID)
		}
		if runtimeMemoryScopeSortKey(a.Scope) != runtimeMemoryScopeSortKey(b.Scope) {
			return strings.Compare(runtimeMemoryScopeSortKey(a.Scope), runtimeMemoryScopeSortKey(b.Scope))
		}
		return strings.Compare(a.Scope, b.Scope)
	})

	slices.SortFunc(catalog.Entries, func(a, b RuntimeMemoryEntryInfo) int {
		if a.Workspace != b.Workspace {
			return strings.Compare(a.Workspace, b.Workspace)
		}
		if runtimeMemoryScopeSortKey(a.Scope) != runtimeMemoryScopeSortKey(b.Scope) {
			return strings.Compare(runtimeMemoryScopeSortKey(a.Scope), runtimeMemoryScopeSortKey(b.Scope))
		}
		if a.AddedAt != b.AddedAt {
			if a.AddedAt < b.AddedAt {
				return -1
			}
			return 1
		}
		if a.Pinned != b.Pinned {
			if a.Pinned {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})

	catalog.Summary.ScopeCount = len(catalog.Scopes)
	catalog.Summary.EntryCount = len(catalog.Entries)
	if len(catalog.Summary.DomainCounts) == 0 {
		catalog.Summary.DomainCounts = nil
	}
	if len(catalog.Summary.EntryTypeCounts) == 0 {
		catalog.Summary.EntryTypeCounts = nil
	}
	if len(catalog.Summary.WorkspaceEntries) == 0 {
		catalog.Summary.WorkspaceEntries = nil
	}

	return catalog
}

func (al *AgentLoop) ExportRuntimeMemoryCatalog(format string) ([]byte, string, string, error) {
	catalog := al.GetRuntimeMemoryCatalog()
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "markdown"
	}

	switch format {
	case "json":
		payload, err := json.MarshalIndent(catalog, "", "  ")
		if err != nil {
			return nil, "", "", err
		}
		return payload, "application/json", "memory-catalog.json", nil
	case "markdown", "md":
		var sb strings.Builder
		sb.WriteString("# PicoClaw Memory Catalog Export\n\n")
		sb.WriteString("- Generated: ")
		sb.WriteString(time.UnixMilli(catalog.GeneratedAt).UTC().Format(time.RFC3339))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("- Workspaces: %d\n", catalog.Summary.WorkspaceCount))
		sb.WriteString(fmt.Sprintf("- Scopes: %d\n", catalog.Summary.ScopeCount))
		sb.WriteString(fmt.Sprintf("- Entries: %d\n", catalog.Summary.EntryCount))
		for _, scope := range catalog.Scopes {
			sb.WriteString("\n## ")
			sb.WriteString(scope.DisplayName)
			sb.WriteString("\n\n")
			sb.WriteString("- Owner Agent: ")
			sb.WriteString(scope.OwnerAgentID)
			sb.WriteString("\n")
			sb.WriteString("- Workspace: ")
			sb.WriteString(scope.Workspace)
			sb.WriteString("\n")
			sb.WriteString("- Scope: ")
			sb.WriteString(scope.Scope)
			sb.WriteString("\n")
			sb.WriteString("- Path: ")
			sb.WriteString(scope.LongTermPath)
			sb.WriteString("\n")
			sb.WriteString("- Entries: ")
			sb.WriteString(fmt.Sprintf("%d", scope.EntryCount))
			sb.WriteString("\n\n")

			raw, err := os.ReadFile(scope.LongTermPath)
			if err != nil || strings.TrimSpace(string(raw)) == "" {
				sb.WriteString("_No approved long-term memory entries in this scope._\n")
				continue
			}
			sb.Write(raw)
			sb.WriteString("\n")
		}
		return []byte(sb.String()), "text/markdown; charset=utf-8", "memory-catalog.md", nil
	default:
		return nil, "", "", fmt.Errorf("%w: unsupported export format %q", ErrRuntimeMemoryCatalogInvalid, format)
	}
}

func runtimeMemoryCatalogWorkspaces(registry *AgentRegistry) []runtimeMemoryWorkspaceRef {
	if registry == nil {
		return nil
	}
	agentIDs := registry.ListAgentIDs()
	slices.Sort(agentIDs)

	seen := make(map[string]bool)
	refs := make([]runtimeMemoryWorkspaceRef, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		agentInst, ok := registry.GetAgent(agentID)
		if !ok || agentInst == nil {
			continue
		}
		workspace := strings.TrimSpace(agentInst.Workspace)
		if workspace == "" || seen[workspace] {
			continue
		}
		seen[workspace] = true
		refs = append(refs, runtimeMemoryWorkspaceRef{
			OwnerAgentID: agentID,
			Workspace:    workspace,
		})
	}
	return refs
}

func runtimeMemoryCatalogScopesForWorkspace(registry *AgentRegistry, ref runtimeMemoryWorkspaceRef) []RuntimeMemoryScopeInfo {
	scopeMap := make(map[string]RuntimeMemoryScopeInfo)
	addScope := func(scope string) {
		mem := NewMemoryStoreForScope(ref.Workspace, scope)
		pathKey := filepath.Clean(mem.LongTermPath())
		if _, exists := scopeMap[pathKey]; exists {
			return
		}
		scopeMap[pathKey] = RuntimeMemoryScopeInfo{
			OwnerAgentID: ref.OwnerAgentID,
			Workspace:    ref.Workspace,
			Scope:        mem.Scope(),
			DisplayName:  mem.DisplayName(),
			LongTermPath: mem.LongTermPath(),
		}
	}

	addScope("shared")

	for _, teammate := range registry.ListTeammates() {
		agentInst, ok := registry.GetAgent(teammate.AgentID)
		if !ok || agentInst == nil {
			continue
		}
		if strings.TrimSpace(agentInst.Workspace) != ref.Workspace {
			continue
		}
		addScope(teammate.MemoryScope)
	}

	memoryRoot := filepath.Join(ref.Workspace, "memory")
	_ = filepath.WalkDir(memoryRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() || d.Name() != "MEMORY.md" {
			return nil
		}
		rel, relErr := filepath.Rel(memoryRoot, path)
		if relErr != nil {
			return nil
		}
		if scope, ok := runtimeMemoryScopeFromRelativePath(rel); ok {
			addScope(scope)
		}
		return nil
	})

	scopes := make([]RuntimeMemoryScopeInfo, 0, len(scopeMap))
	for _, scope := range scopeMap {
		scopes = append(scopes, scope)
	}
	return scopes
}

func runtimeMemoryScopeFromRelativePath(rel string) (string, bool) {
	rel = filepath.ToSlash(filepath.Clean(rel))
	switch {
	case rel == "MEMORY.md":
		return "shared", true
	case strings.HasPrefix(rel, "teammates/") && strings.HasSuffix(rel, "/MEMORY.md"):
		scope := strings.TrimSuffix(strings.TrimPrefix(rel, "teammates/"), "/MEMORY.md")
		scope = strings.Trim(scope, "/")
		if scope == "" {
			return "", false
		}
		return "teammate:" + scope, true
	case strings.HasPrefix(rel, "scopes/") && strings.HasSuffix(rel, "/MEMORY.md"):
		scope := strings.TrimSuffix(strings.TrimPrefix(rel, "scopes/"), "/MEMORY.md")
		scope = strings.Trim(scope, "/")
		if scope == "" {
			return "", false
		}
		return scope, true
	default:
		return "", false
	}
}

func runtimeMemoryScopeSortKey(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "shared" {
		return "0:shared"
	}
	if strings.HasPrefix(scope, "teammate:") {
		return "1:" + scope
	}
	return "2:" + scope
}

func parseRuntimeMemoryCatalogEntries(ownerAgentID, workspace string, mem *MemoryStore, content string) []RuntimeMemoryEntryInfo {
	if mem == nil {
		return nil
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	preamble, sections := splitRuntimeMemoryCatalogSections(trimmed)
	parsed := make([]runtimeMemoryParsedSection, 0, len(sections)+1)
	if preamble != "" {
		parsed = append(parsed, runtimeMemoryParsedSection{
			Title:   "Legacy Memory",
			Content: preamble,
			Domain:  defaultMemoryProposalDomainForScope(mem.Scope()),
			Legacy:  true,
		})
	}
	if len(sections) == 0 {
		if len(parsed) == 0 {
			parsed = append(parsed, runtimeMemoryParsedSection{
				Title:   "Legacy Memory",
				Content: trimmed,
				Domain:  defaultMemoryProposalDomainForScope(mem.Scope()),
				Legacy:  true,
			})
		}
	} else {
		for _, section := range sections {
			parsed = append(parsed, parseRuntimeMemoryCatalogSection(section.title, section.body, mem.Scope()))
		}
	}

	entries := make([]RuntimeMemoryEntryInfo, 0, len(parsed))
	baseIDs := make([]string, len(parsed))
	baseIDCounts := make(map[string]int, len(parsed))
	for i, section := range parsed {
		baseID := runtimeMemoryCatalogEntryBaseID(workspace, mem.LongTermPath(), section)
		baseIDs[i] = baseID
		baseIDCounts[baseID]++
	}
	baseIDSeen := make(map[string]int, len(baseIDCounts))
	for i, section := range parsed {
		baseID := baseIDs[i]
		baseIDSeen[baseID]++
		entry := RuntimeMemoryEntryInfo{
			ID:               runtimeMemoryCatalogEntryID(baseID, baseIDSeen[baseID], baseIDCounts[baseID]),
			OwnerAgentID:     ownerAgentID,
			Workspace:        workspace,
			Scope:            mem.Scope(),
			ScopeDisplayName: mem.DisplayName(),
			SourcePath:       mem.LongTermPath(),
			Title:            strings.TrimSpace(section.Title),
			Content:          strings.TrimSpace(section.Content),
			Domain:           strings.TrimSpace(section.Domain),
			EntryType:        strings.TrimSpace(section.EntryType),
			Confidence:       strings.TrimSpace(section.Confidence),
			AddedAt:          section.AddedAt,
			AddedAtDisplay:   strings.TrimSpace(section.AddedAtDisplay),
			SourceTaskID:     strings.TrimSpace(section.SourceTaskID),
			SourceTeammateID: strings.TrimSpace(section.SourceTeammate),
			ReviewedBy:       strings.TrimSpace(section.ReviewedBy),
			Legacy:           section.Legacy,
		}
		if entry.Title == "" {
			entry.Title = "Reviewed Memory Entry"
		}
		if entry.Domain == "" {
			entry.Domain = defaultMemoryProposalDomainForScope(mem.Scope())
		}
		entries = append(entries, entry)
	}
	return entries
}

func runtimeMemoryCatalogEntryBaseID(
	workspace, sourcePath string,
	section runtimeMemoryParsedSection,
) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		workspace,
		sourcePath,
		section.Title,
		section.Content,
		section.Domain,
		section.EntryType,
		section.Confidence,
		section.AddedAtDisplay,
		section.SourceTaskID,
		section.SourceTeammate,
		section.ReviewedBy,
	}, "\x00")))
	return "memory-" + hex.EncodeToString(sum[:12])
}

func runtimeMemoryCatalogEntryID(baseID string, occurrence, total int) string {
	if total <= 1 {
		return baseID
	}
	return fmt.Sprintf("%s-dup-%d", baseID, occurrence)
}

type runtimeMemoryCatalogSection struct {
	title string
	body  string
}

func splitRuntimeMemoryCatalogSections(content string) (string, []runtimeMemoryCatalogSection) {
	lines := strings.Split(content, "\n")
	preamble := make([]string, 0)
	sections := make([]runtimeMemoryCatalogSection, 0)
	currentTitle := ""
	currentLines := make([]string, 0)

	flush := func() {
		if strings.TrimSpace(currentTitle) == "" {
			return
		}
		sections = append(sections, runtimeMemoryCatalogSection{
			title: strings.TrimSpace(currentTitle),
			body:  strings.TrimSpace(strings.Join(currentLines, "\n")),
		})
		currentTitle = ""
		currentLines = currentLines[:0]
	}

	for idx, line := range lines {
		if runtimeMemoryCatalogSectionStartsAt(lines, idx) {
			flush()
			currentTitle = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		if currentTitle == "" {
			preamble = append(preamble, line)
			continue
		}
		currentLines = append(currentLines, line)
	}
	flush()
	return strings.TrimSpace(strings.Join(preamble, "\n")), sections
}

func runtimeMemoryCatalogSectionStartsAt(lines []string, idx int) bool {
	if idx < 0 || idx >= len(lines) {
		return false
	}
	if !strings.HasPrefix(lines[idx], "## ") {
		return false
	}
	next := idx + 1
	for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
		next++
	}
	if next >= len(lines) {
		return false
	}
	key, _, ok := parseRuntimeMemoryMetadataLine(strings.TrimSpace(lines[next]))
	return ok && key == "Added"
}

func parseRuntimeMemoryCatalogSection(title, body, scope string) runtimeMemoryParsedSection {
	parsed := runtimeMemoryParsedSection{
		Title:   strings.TrimSpace(title),
		Domain:  defaultMemoryProposalDomainForScope(scope),
		Content: strings.TrimSpace(body),
	}
	if strings.TrimSpace(body) == "" {
		return parsed
	}

	lines := strings.Split(body, "\n")
	metadata := make(map[string]string)
	contentStart := 0
	metadataPhase := true
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !metadataPhase {
			break
		}
		if trimmed == "" {
			contentStart = idx + 1
			metadataPhase = false
			continue
		}
		key, value, ok := parseRuntimeMemoryMetadataLine(trimmed)
		if !ok {
			contentStart = idx
			metadataPhase = false
			break
		}
		metadata[key] = value
		contentStart = idx + 1
	}

	parsed.Content = strings.TrimSpace(strings.Join(lines[contentStart:], "\n"))
	if parsed.Content == "" && len(metadata) == 0 {
		parsed.Legacy = true
		parsed.Content = strings.TrimSpace(body)
		return parsed
	}
	if value := strings.TrimSpace(metadata["Added"]); value != "" {
		parsed.AddedAtDisplay = value
		if ts, err := time.Parse("2006-01-02 15:04:05 MST", value); err == nil {
			parsed.AddedAt = ts.UnixMilli()
		}
	}
	if value := strings.TrimSpace(metadata["Domain"]); value != "" {
		parsed.Domain = value
	}
	parsed.EntryType = strings.TrimSpace(metadata["Type"])
	parsed.Confidence = strings.TrimSpace(metadata["Confidence"])
	parsed.SourceTaskID = strings.TrimSpace(metadata["Source Task"])
	parsed.SourceTeammate = strings.TrimSpace(metadata["Source Teammate"])
	parsed.ReviewedBy = strings.TrimSpace(metadata["Reviewed By"])
	return parsed
}

func parseRuntimeMemoryMetadataLine(line string) (string, string, bool) {
	if !strings.HasPrefix(line, "- ") {
		return "", "", false
	}
	key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), ":")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}
