package agent

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type RuntimeMemoryCatalogQuery struct {
	Search       string
	Scope        string
	Domain       string
	EntryType    string
	Archive      string
	OwnerAgentID string
	Limit        int
}

type RuntimeMemoryHistoryQuery struct {
	Search       string
	Kind         string
	Scope        string
	Actor        string
	OwnerAgentID string
	Limit        int
}

type RuntimeMemoryHistory struct {
	GeneratedAt int64                       `json:"generated_at"`
	Summary     RuntimeMemoryHistoryStats   `json:"summary"`
	Events      []RuntimeMemoryHistoryEvent `json:"events,omitempty"`
}

type RuntimeMemoryHistoryStats struct {
	EventCount         int            `json:"event_count"`
	CatalogEventCount  int            `json:"catalog_event_count"`
	ProposalEventCount int            `json:"proposal_event_count"`
	KindCounts         map[string]int `json:"kind_counts,omitempty"`
}

type RuntimeMemoryHistoryEvent struct {
	ID               string `json:"id"`
	Kind             string `json:"kind"`
	OwnerAgentID     string `json:"owner_agent_id"`
	Workspace        string `json:"workspace"`
	Scope            string `json:"scope,omitempty"`
	ScopeDisplayName string `json:"scope_display_name,omitempty"`
	SubjectID        string `json:"subject_id"`
	SubjectType      string `json:"subject_type"`
	Title            string `json:"title,omitempty"`
	Content          string `json:"content,omitempty"`
	Domain           string `json:"domain,omitempty"`
	EntryType        string `json:"entry_type,omitempty"`
	Status           string `json:"status,omitempty"`
	Actor            string `json:"actor,omitempty"`
	Timestamp        int64  `json:"timestamp"`
}

func (al *AgentLoop) SearchRuntimeMemoryCatalog(query RuntimeMemoryCatalogQuery) RuntimeMemoryCatalog {
	catalog := al.GetRuntimeMemoryCatalog()
	filteredEntries := make([]RuntimeMemoryEntryInfo, 0, len(catalog.Entries))
	for _, entry := range catalog.Entries {
		if runtimeMemoryCatalogEntryMatchesQuery(entry, query) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	if query.Limit > 0 && len(filteredEntries) > query.Limit {
		filteredEntries = filteredEntries[:query.Limit]
	}

	scopeMap := make(map[string]RuntimeMemoryScopeInfo)
	summary := RuntimeMemoryCatalogStats{
		DomainCounts:     map[string]int{},
		EntryTypeCounts:  map[string]int{},
		WorkspaceEntries: map[string]int{},
	}
	for _, entry := range filteredEntries {
		scopeKey := fmt.Sprintf("%s:%s:%s", entry.OwnerAgentID, entry.Workspace, entry.Scope)
		if _, ok := scopeMap[scopeKey]; !ok {
			scopeMap[scopeKey] = RuntimeMemoryScopeInfo{
				OwnerAgentID: entry.OwnerAgentID,
				Workspace:    entry.Workspace,
				Scope:        entry.Scope,
				DisplayName:  entry.ScopeDisplayName,
				LongTermPath: entry.SourcePath,
				HasLongTerm:  true,
			}
		}
		scopeInfo := scopeMap[scopeKey]
		scopeInfo.EntryCount++
		scopeMap[scopeKey] = scopeInfo

		summary.WorkspaceEntries[entry.Workspace]++
		if domain := strings.TrimSpace(entry.Domain); domain != "" {
			summary.DomainCounts[domain]++
		}
		if entryType := strings.TrimSpace(entry.EntryType); entryType != "" {
			summary.EntryTypeCounts[entryType]++
		}
		if entry.Pinned {
			summary.PinnedCount++
		}
		if entry.Archived {
			summary.ArchivedCount++
		}
	}

	filteredScopes := make([]RuntimeMemoryScopeInfo, 0, len(scopeMap))
	for _, scopeInfo := range scopeMap {
		filteredScopes = append(filteredScopes, scopeInfo)
	}
	slices.SortFunc(filteredScopes, func(a, b RuntimeMemoryScopeInfo) int {
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

	summary.WorkspaceCount = len(summary.WorkspaceEntries)
	summary.ScopeCount = len(filteredScopes)
	summary.EntryCount = len(filteredEntries)
	if len(summary.DomainCounts) == 0 {
		summary.DomainCounts = nil
	}
	if len(summary.EntryTypeCounts) == 0 {
		summary.EntryTypeCounts = nil
	}
	if len(summary.WorkspaceEntries) == 0 {
		summary.WorkspaceEntries = nil
	}

	return RuntimeMemoryCatalog{
		GeneratedAt: catalog.GeneratedAt,
		Summary:     summary,
		Scopes:      filteredScopes,
		Entries:     filteredEntries,
	}
}

func (al *AgentLoop) GetRuntimeMemoryHistory(query RuntimeMemoryHistoryQuery) RuntimeMemoryHistory {
	history := RuntimeMemoryHistory{
		GeneratedAt: time.Now().UnixMilli(),
		Summary: RuntimeMemoryHistoryStats{
			KindCounts: map[string]int{},
		},
	}

	catalog := al.GetRuntimeMemoryCatalog()
	events := make([]RuntimeMemoryHistoryEvent, 0, len(catalog.Entries)*3)
	for _, entry := range catalog.Entries {
		if entry.AddedAt > 0 {
			events = append(events, runtimeMemoryHistoryEventFromCatalogEntry(entry, "entry_added", entry.ReviewedBy, entry.AddedAt))
		}
		if entry.PinnedAt > 0 {
			events = append(events, runtimeMemoryHistoryEventFromCatalogEntry(entry, "entry_pinned", entry.PinnedBy, entry.PinnedAt))
		}
		if entry.ArchivedAt > 0 {
			events = append(events, runtimeMemoryHistoryEventFromCatalogEntry(entry, "entry_archived", entry.ArchivedBy, entry.ArchivedAt))
		}
	}

	for _, proposal := range al.GetRuntimeSnapshot().MemoryProposals {
		if proposal.Created > 0 {
			events = append(events, runtimeMemoryHistoryEventFromProposal(proposal, "proposal_created", runtimeMemoryProposalActor(proposal), proposal.Created))
		}
		if proposal.UpdatedAt > 0 {
			events = append(events, runtimeMemoryHistoryEventFromProposal(proposal, "proposal_updated", proposal.UpdatedBy, proposal.UpdatedAt))
		}
		if proposal.ReviewedAt > 0 {
			kind := "proposal_reviewed"
			switch strings.ToLower(strings.TrimSpace(proposal.Status)) {
			case "approved":
				kind = "proposal_approved"
			case "rejected":
				kind = "proposal_rejected"
			}
			events = append(events, runtimeMemoryHistoryEventFromProposal(proposal, kind, proposal.ReviewedBy, proposal.ReviewedAt))
		}
	}

	filtered := make([]RuntimeMemoryHistoryEvent, 0, len(events))
	for _, event := range events {
		if runtimeMemoryHistoryEventMatchesQuery(event, query) {
			filtered = append(filtered, event)
		}
	}

	slices.SortFunc(filtered, func(a, b RuntimeMemoryHistoryEvent) int {
		if a.Timestamp != b.Timestamp {
			if a.Timestamp > b.Timestamp {
				return -1
			}
			return 1
		}
		if a.Kind != b.Kind {
			return strings.Compare(a.Kind, b.Kind)
		}
		return strings.Compare(a.ID, b.ID)
	})
	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[:query.Limit]
	}

	for _, event := range filtered {
		history.Summary.KindCounts[event.Kind]++
		switch event.SubjectType {
		case "catalog_entry":
			history.Summary.CatalogEventCount++
		case "memory_proposal":
			history.Summary.ProposalEventCount++
		}
	}
	history.Summary.EventCount = len(filtered)
	if len(history.Summary.KindCounts) == 0 {
		history.Summary.KindCounts = nil
	}
	history.Events = filtered
	return history
}

func runtimeMemoryCatalogEntryMatchesQuery(entry RuntimeMemoryEntryInfo, query RuntimeMemoryCatalogQuery) bool {
	switch archive := strings.ToLower(strings.TrimSpace(query.Archive)); archive {
	case "", "all":
	case "active":
		if entry.Archived {
			return false
		}
	case "archived":
		if !entry.Archived {
			return false
		}
	default:
		return false
	}
	if scope := strings.TrimSpace(query.Scope); scope != "" && entry.Scope != scope {
		return false
	}
	if domain := strings.TrimSpace(query.Domain); domain != "" && entry.Domain != domain {
		return false
	}
	if entryType := strings.TrimSpace(query.EntryType); entryType != "" && entry.EntryType != entryType {
		return false
	}
	if ownerAgentID := strings.TrimSpace(query.OwnerAgentID); ownerAgentID != "" && entry.OwnerAgentID != ownerAgentID {
		return false
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	if search == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.Join([]string{
		entry.ID,
		entry.OwnerAgentID,
		entry.Scope,
		entry.ScopeDisplayName,
		entry.Title,
		entry.Content,
		entry.Domain,
		entry.EntryType,
		entry.SourceTaskID,
		entry.SourceTeammateID,
		entry.SourcePath,
	}, "\n")), search)
}

func runtimeMemoryHistoryEventFromCatalogEntry(
	entry RuntimeMemoryEntryInfo,
	kind, actor string,
	timestamp int64,
) RuntimeMemoryHistoryEvent {
	return RuntimeMemoryHistoryEvent{
		ID:               fmt.Sprintf("%s:%s:%d", kind, entry.ID, timestamp),
		Kind:             kind,
		OwnerAgentID:     entry.OwnerAgentID,
		Workspace:        entry.Workspace,
		Scope:            entry.Scope,
		ScopeDisplayName: entry.ScopeDisplayName,
		SubjectID:        entry.ID,
		SubjectType:      "catalog_entry",
		Title:            entry.Title,
		Content:          entry.Content,
		Domain:           entry.Domain,
		EntryType:        entry.EntryType,
		Actor:            strings.TrimSpace(actor),
		Timestamp:        timestamp,
	}
}

func runtimeMemoryHistoryEventFromProposal(
	proposal RuntimeMemoryProposalInfo,
	kind, actor string,
	timestamp int64,
) RuntimeMemoryHistoryEvent {
	return RuntimeMemoryHistoryEvent{
		ID:               fmt.Sprintf("%s:%s:%d", kind, proposal.ID, timestamp),
		Kind:             kind,
		OwnerAgentID:     proposal.OwnerAgentID,
		Scope:            proposal.Scope,
		ScopeDisplayName: NewMemoryStoreForScope(".", proposal.Scope).DisplayName(),
		SubjectID:        proposal.ID,
		SubjectType:      "memory_proposal",
		Title:            proposal.Title,
		Content:          proposal.Content,
		Domain:           proposal.Domain,
		EntryType:        proposal.EntryType,
		Status:           proposal.Status,
		Actor:            strings.TrimSpace(actor),
		Timestamp:        timestamp,
	}
}

func runtimeMemoryProposalActor(proposal RuntimeMemoryProposalInfo) string {
	switch {
	case strings.TrimSpace(proposal.RequesterTeammateID) != "":
		return proposal.RequesterTeammateID
	case strings.TrimSpace(proposal.RequesterAgentID) != "":
		return proposal.RequesterAgentID
	case strings.TrimSpace(proposal.SourceTeammateID) != "":
		return proposal.SourceTeammateID
	case strings.TrimSpace(proposal.SourceAgentID) != "":
		return proposal.SourceAgentID
	default:
		return ""
	}
}

func runtimeMemoryHistoryEventMatchesQuery(event RuntimeMemoryHistoryEvent, query RuntimeMemoryHistoryQuery) bool {
	if kind := strings.TrimSpace(query.Kind); kind != "" && event.Kind != kind {
		return false
	}
	if scope := strings.TrimSpace(query.Scope); scope != "" && event.Scope != scope {
		return false
	}
	if actor := strings.TrimSpace(query.Actor); actor != "" && event.Actor != actor {
		return false
	}
	if ownerAgentID := strings.TrimSpace(query.OwnerAgentID); ownerAgentID != "" && event.OwnerAgentID != ownerAgentID {
		return false
	}
	search := strings.ToLower(strings.TrimSpace(query.Search))
	if search == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.Join([]string{
		event.Kind,
		event.OwnerAgentID,
		event.Scope,
		event.ScopeDisplayName,
		event.SubjectID,
		event.SubjectType,
		event.Title,
		event.Content,
		event.Domain,
		event.EntryType,
		event.Status,
		event.Actor,
	}, "\n")), search)
}
