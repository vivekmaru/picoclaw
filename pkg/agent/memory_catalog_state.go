package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/fileutil"
)

type runtimeMemoryCatalogEntryState struct {
	ID         string `json:"id"`
	Pinned     bool   `json:"pinned,omitempty"`
	PinnedAt   int64  `json:"pinned_at,omitempty"`
	PinnedBy   string `json:"pinned_by,omitempty"`
	Archived   bool   `json:"archived,omitempty"`
	ArchivedAt int64  `json:"archived_at,omitempty"`
	ArchivedBy string `json:"archived_by,omitempty"`
}

type runtimeMemoryCatalogStateFile struct {
	Version int                              `json:"version"`
	Entries []runtimeMemoryCatalogEntryState `json:"entries"`
}

type runtimeMemoryCatalogStateStore struct {
	stateFile string
	entries   map[string]*runtimeMemoryCatalogEntryState
	mu        sync.RWMutex
}

const runtimeMemoryCatalogStateVersion = 1

var runtimeMemoryCatalogStateStores sync.Map

func getRuntimeMemoryCatalogStateStore(workspace string) *runtimeMemoryCatalogStateStore {
	workspace = filepath.Clean(strings.TrimSpace(workspace))
	if workspace == "" {
		return &runtimeMemoryCatalogStateStore{
			stateFile: filepath.Join("state", "memory", "catalog_state.json"),
			entries:   make(map[string]*runtimeMemoryCatalogEntryState),
		}
	}
	if existing, ok := runtimeMemoryCatalogStateStores.Load(workspace); ok {
		return existing.(*runtimeMemoryCatalogStateStore)
	}
	store := &runtimeMemoryCatalogStateStore{
		stateFile: filepath.Join(workspace, "state", "memory", "catalog_state.json"),
		entries:   make(map[string]*runtimeMemoryCatalogEntryState),
	}
	_ = store.load()
	actual, _ := runtimeMemoryCatalogStateStores.LoadOrStore(workspace, store)
	return actual.(*runtimeMemoryCatalogStateStore)
}

func (s *runtimeMemoryCatalogStateStore) getCopy(id string) (runtimeMemoryCatalogEntryState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[id]
	if !ok {
		return runtimeMemoryCatalogEntryState{}, false
	}
	return *entry, true
}

func (s *runtimeMemoryCatalogStateStore) apply(entry *RuntimeMemoryEntryInfo) {
	if s == nil || entry == nil {
		return
	}
	state, ok := s.getCopy(entry.ID)
	if !ok {
		return
	}
	entry.Pinned = state.Pinned
	entry.PinnedAt = state.PinnedAt
	entry.PinnedBy = state.PinnedBy
	entry.Archived = state.Archived
	entry.ArchivedAt = state.ArchivedAt
	entry.ArchivedBy = state.ArchivedBy
}

func (s *runtimeMemoryCatalogStateStore) setPinned(id string, pinned bool, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.ensureEntryLocked(id)
	entry.Pinned = pinned
	if pinned {
		entry.PinnedAt = time.Now().UnixMilli()
		entry.PinnedBy = defaultReviewActor(actor)
	} else {
		entry.PinnedAt = 0
		entry.PinnedBy = ""
	}
	return s.persistLocked()
}

func (s *runtimeMemoryCatalogStateStore) setArchived(id string, archived bool, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.ensureEntryLocked(id)
	entry.Archived = archived
	if archived {
		entry.ArchivedAt = time.Now().UnixMilli()
		entry.ArchivedBy = defaultReviewActor(actor)
	} else {
		entry.ArchivedAt = 0
		entry.ArchivedBy = ""
	}
	return s.persistLocked()
}

func (s *runtimeMemoryCatalogStateStore) ensureEntryLocked(id string) *runtimeMemoryCatalogEntryState {
	if entry, ok := s.entries[id]; ok {
		return entry
	}
	entry := &runtimeMemoryCatalogEntryState{ID: id}
	s.entries[id] = entry
	return entry
}

func (s *runtimeMemoryCatalogStateStore) load() error {
	data, err := os.ReadFile(s.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload runtimeMemoryCatalogStateFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if payload.Version != 0 && payload.Version != runtimeMemoryCatalogStateVersion {
		return fmt.Errorf("unsupported memory catalog state version: got %d, want %d", payload.Version, runtimeMemoryCatalogStateVersion)
	}
	for i := range payload.Entries {
		entry := payload.Entries[i]
		if strings.TrimSpace(entry.ID) == "" {
			continue
		}
		s.entries[entry.ID] = &entry
	}
	return nil
}

func (s *runtimeMemoryCatalogStateStore) persistLocked() error {
	entries := make([]runtimeMemoryCatalogEntryState, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry == nil || strings.TrimSpace(entry.ID) == "" {
			continue
		}
		if !entry.Pinned && !entry.Archived {
			continue
		}
		entries = append(entries, *entry)
	}
	slices.SortFunc(entries, func(a, b runtimeMemoryCatalogEntryState) int {
		return strings.Compare(a.ID, b.ID)
	})
	payload, err := json.MarshalIndent(runtimeMemoryCatalogStateFile{
		Version: runtimeMemoryCatalogStateVersion,
		Entries: entries,
	}, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(s.stateFile, payload, 0o600)
}
