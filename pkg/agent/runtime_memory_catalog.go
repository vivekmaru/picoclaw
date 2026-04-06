package agent

import "fmt"

func (al *AgentLoop) PinRuntimeMemoryCatalogEntry(entryID, actor string) (RuntimeMemoryEntryInfo, error) {
	return al.setRuntimeMemoryCatalogEntryPinned(entryID, actor, true)
}

func (al *AgentLoop) UnpinRuntimeMemoryCatalogEntry(entryID, actor string) (RuntimeMemoryEntryInfo, error) {
	return al.setRuntimeMemoryCatalogEntryPinned(entryID, actor, false)
}

func (al *AgentLoop) ArchiveRuntimeMemoryCatalogEntry(entryID, actor string) (RuntimeMemoryEntryInfo, error) {
	return al.setRuntimeMemoryCatalogEntryArchived(entryID, actor, true)
}

func (al *AgentLoop) RestoreRuntimeMemoryCatalogEntry(entryID, actor string) (RuntimeMemoryEntryInfo, error) {
	return al.setRuntimeMemoryCatalogEntryArchived(entryID, actor, false)
}

func (al *AgentLoop) setRuntimeMemoryCatalogEntryPinned(entryID, actor string, pinned bool) (RuntimeMemoryEntryInfo, error) {
	entry, err := al.resolveRuntimeMemoryCatalogEntry(entryID)
	if err != nil {
		return RuntimeMemoryEntryInfo{}, err
	}
	store := newRuntimeMemoryCatalogStateStore(entry.Workspace)
	if err := store.setPinned(entry.ID, pinned, actor); err != nil {
		return RuntimeMemoryEntryInfo{}, err
	}
	return al.resolveRuntimeMemoryCatalogEntry(entry.ID)
}

func (al *AgentLoop) setRuntimeMemoryCatalogEntryArchived(entryID, actor string, archived bool) (RuntimeMemoryEntryInfo, error) {
	entry, err := al.resolveRuntimeMemoryCatalogEntry(entryID)
	if err != nil {
		return RuntimeMemoryEntryInfo{}, err
	}
	store := newRuntimeMemoryCatalogStateStore(entry.Workspace)
	if err := store.setArchived(entry.ID, archived, actor); err != nil {
		return RuntimeMemoryEntryInfo{}, err
	}
	return al.resolveRuntimeMemoryCatalogEntry(entry.ID)
}

func (al *AgentLoop) resolveRuntimeMemoryCatalogEntry(entryID string) (RuntimeMemoryEntryInfo, error) {
	catalog := al.GetRuntimeMemoryCatalog()
	for _, entry := range catalog.Entries {
		if entry.ID == entryID {
			return entry, nil
		}
	}
	return RuntimeMemoryEntryInfo{}, fmt.Errorf("%w: %s", ErrRuntimeMemoryCatalogEntryNotFound, entryID)
}
