package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/fileutil"
)

var (
	errMemoryProposalNotPending = errors.New("memory proposal not pending")
	errMemoryProposalInvalid    = errors.New("invalid memory proposal")
)

type MemoryProposal struct {
	ID                  string `json:"id"`
	Scope               string `json:"scope"`
	Target              string `json:"target"`
	Kind                string `json:"kind"`
	Status              string `json:"status"`
	Title               string `json:"title,omitempty"`
	Content             string `json:"content"`
	SourceTaskID        string `json:"source_task_id,omitempty"`
	SourceAgentID       string `json:"source_agent_id,omitempty"`
	SourceTeammateID    string `json:"source_teammate_id,omitempty"`
	RequesterAgentID    string `json:"requester_agent_id,omitempty"`
	RequesterTeammateID string `json:"requester_teammate_id,omitempty"`
	Created             int64  `json:"created"`
	UpdatedAt           int64  `json:"updated_at,omitempty"`
	UpdatedBy           string `json:"updated_by,omitempty"`
	ReviewedAt          int64  `json:"reviewed_at,omitempty"`
	ReviewedBy          string `json:"reviewed_by,omitempty"`
	ReviewNote          string `json:"review_note,omitempty"`
}

type MemoryProposalRequest struct {
	Scope               string
	Target              string
	Kind                string
	Title               string
	Content             string
	SourceTaskID        string
	SourceAgentID       string
	SourceTeammateID    string
	RequesterAgentID    string
	RequesterTeammateID string
}

type MemoryProposalUpdate struct {
	Scope   string
	Title   string
	Content string
}

type memoryProposalStoreFile struct {
	Version   int              `json:"version"`
	NextID    int              `json:"next_id"`
	Proposals []MemoryProposal `json:"proposals"`
}

type MemoryProposalStore struct {
	workspace string
	stateFile string
	nextID    int
	proposals map[string]*MemoryProposal
	mu        sync.RWMutex
}

const memoryProposalStoreVersion = 1

func NewMemoryProposalStore(workspace string) *MemoryProposalStore {
	store := &MemoryProposalStore{
		workspace: workspace,
		stateFile: filepath.Join(workspace, "state", "memory", "proposals.json"),
		nextID:    1,
		proposals: make(map[string]*MemoryProposal),
	}
	_ = store.load()
	return store
}

func (s *MemoryProposalStore) Create(req MemoryProposalRequest) (MemoryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return MemoryProposal{}, fmt.Errorf("proposal content is required")
	}

	scope := strings.TrimSpace(req.Scope)
	if scope == "" {
		scope = "shared"
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = "long_term"
	}
	if target != "long_term" {
		return MemoryProposal{}, fmt.Errorf("unsupported memory proposal target %q", target)
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "task_result"
	}

	id := fmt.Sprintf("memory-%d", s.nextID)
	s.nextID++
	proposal := &MemoryProposal{
		ID:                  id,
		Scope:               scope,
		Target:              target,
		Kind:                kind,
		Status:              "pending",
		Title:               strings.TrimSpace(req.Title),
		Content:             content,
		SourceTaskID:        strings.TrimSpace(req.SourceTaskID),
		SourceAgentID:       strings.TrimSpace(req.SourceAgentID),
		SourceTeammateID:    strings.TrimSpace(req.SourceTeammateID),
		RequesterAgentID:    strings.TrimSpace(req.RequesterAgentID),
		RequesterTeammateID: strings.TrimSpace(req.RequesterTeammateID),
		Created:             time.Now().UnixMilli(),
	}
	s.proposals[id] = proposal
	if err := s.persistLocked(); err != nil {
		delete(s.proposals, id)
		s.nextID--
		return MemoryProposal{}, err
	}
	return *proposal, nil
}

func (s *MemoryProposalStore) GetCopy(id string) (MemoryProposal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	proposal, ok := s.proposals[id]
	if !ok {
		return MemoryProposal{}, false
	}
	return *proposal, true
}

func (s *MemoryProposalStore) ListCopies() []MemoryProposal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	proposals := make([]MemoryProposal, 0, len(s.proposals))
	for _, proposal := range s.proposals {
		proposals = append(proposals, *proposal)
	}
	slices.SortFunc(proposals, func(a, b MemoryProposal) int {
		if a.Created != b.Created {
			if a.Created < b.Created {
				return -1
			}
			return 1
		}
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	return proposals
}

func (s *MemoryProposalStore) Approve(id, actor, note string) (MemoryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	proposal, ok := s.proposals[id]
	if !ok {
		return MemoryProposal{}, fmt.Errorf("memory proposal %q not found", id)
	}
	if proposal.Status != "pending" {
		return MemoryProposal{}, fmt.Errorf("%w: memory proposal %q is not pending", errMemoryProposalNotPending, id)
	}

	mem := NewMemoryStoreForScope(s.workspace, proposal.Scope)
	if err := appendMemoryProposal(mem, *proposal); err != nil {
		return MemoryProposal{}, err
	}

	proposal.Status = "approved"
	proposal.ReviewedAt = time.Now().UnixMilli()
	proposal.ReviewedBy = defaultReviewActor(actor)
	proposal.ReviewNote = strings.TrimSpace(note)
	if err := s.persistLocked(); err != nil {
		return MemoryProposal{}, err
	}
	return *proposal, nil
}

func (s *MemoryProposalStore) Update(id, actor string, update MemoryProposalUpdate) (MemoryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	proposal, ok := s.proposals[id]
	if !ok {
		return MemoryProposal{}, fmt.Errorf("memory proposal %q not found", id)
	}
	if proposal.Status != "pending" {
		return MemoryProposal{}, fmt.Errorf("%w: memory proposal %q is not pending", errMemoryProposalNotPending, id)
	}

	scope := strings.TrimSpace(update.Scope)
	if scope == "" {
		return MemoryProposal{}, fmt.Errorf("%w: memory proposal scope is required", errMemoryProposalInvalid)
	}
	content := strings.TrimSpace(update.Content)
	if content == "" {
		return MemoryProposal{}, fmt.Errorf("%w: memory proposal content is required", errMemoryProposalInvalid)
	}

	proposal.Scope = scope
	proposal.Title = strings.TrimSpace(update.Title)
	proposal.Content = content
	proposal.UpdatedAt = time.Now().UnixMilli()
	proposal.UpdatedBy = defaultReviewActor(actor)
	if err := s.persistLocked(); err != nil {
		return MemoryProposal{}, err
	}
	return *proposal, nil
}

func (s *MemoryProposalStore) Reject(id, actor, note string) (MemoryProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	proposal, ok := s.proposals[id]
	if !ok {
		return MemoryProposal{}, fmt.Errorf("memory proposal %q not found", id)
	}
	if proposal.Status != "pending" {
		return MemoryProposal{}, fmt.Errorf("%w: memory proposal %q is not pending", errMemoryProposalNotPending, id)
	}

	proposal.Status = "rejected"
	proposal.ReviewedAt = time.Now().UnixMilli()
	proposal.ReviewedBy = defaultReviewActor(actor)
	proposal.ReviewNote = strings.TrimSpace(note)
	if proposal.ReviewNote == "" {
		proposal.ReviewNote = "Proposal rejected during review"
	}
	if err := s.persistLocked(); err != nil {
		return MemoryProposal{}, err
	}
	return *proposal, nil
}

func (s *MemoryProposalStore) load() error {
	data, err := os.ReadFile(s.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var payload memoryProposalStoreFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	if payload.Version != 0 && payload.Version != memoryProposalStoreVersion {
		return fmt.Errorf(
			"unsupported memory proposal store version: got %d, want %d",
			payload.Version,
			memoryProposalStoreVersion,
		)
	}
	maxID := 0
	for i := range payload.Proposals {
		proposal := payload.Proposals[i]
		s.proposals[proposal.ID] = &proposal
		if parsed := parseMemoryProposalNumericID(proposal.ID); parsed > maxID {
			maxID = parsed
		}
	}
	if payload.NextID > maxID {
		s.nextID = payload.NextID
	} else if maxID > 0 {
		s.nextID = maxID + 1
	}
	return nil
}

func (s *MemoryProposalStore) persistLocked() error {
	proposals := make([]MemoryProposal, 0, len(s.proposals))
	for _, proposal := range s.proposals {
		proposals = append(proposals, *proposal)
	}
	slices.SortFunc(proposals, func(a, b MemoryProposal) int {
		if a.Created != b.Created {
			if a.Created < b.Created {
				return -1
			}
			return 1
		}
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	payload, err := json.MarshalIndent(memoryProposalStoreFile{
		Version:   memoryProposalStoreVersion,
		NextID:    s.nextID,
		Proposals: proposals,
	}, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(s.stateFile, payload, 0o600)
}

func appendMemoryProposal(mem *MemoryStore, proposal MemoryProposal) error {
	if mem == nil {
		return fmt.Errorf("memory store unavailable")
	}
	entry := formatMemoryProposalEntry(proposal)
	existing := strings.TrimRight(mem.ReadLongTerm(), "\n")
	if existing == "" {
		return mem.WriteLongTerm(entry)
	}
	return mem.WriteLongTerm(existing + "\n\n" + entry)
}

func formatMemoryProposalEntry(proposal MemoryProposal) string {
	var sb strings.Builder
	title := proposal.Title
	if strings.TrimSpace(title) == "" {
		title = "Reviewed Memory Entry"
	}
	sb.WriteString("## ")
	sb.WriteString(title)
	sb.WriteString("\n\n")
	sb.WriteString("- Added: ")
	sb.WriteString(time.UnixMilli(proposal.Created).UTC().Format("2006-01-02 15:04:05 UTC"))
	sb.WriteString("\n")
	if proposal.SourceTaskID != "" {
		sb.WriteString("- Source Task: ")
		sb.WriteString(proposal.SourceTaskID)
		sb.WriteString("\n")
	}
	if proposal.SourceTeammateID != "" {
		sb.WriteString("- Source Teammate: ")
		sb.WriteString(proposal.SourceTeammateID)
		sb.WriteString("\n")
	}
	if proposal.ReviewedBy != "" {
		sb.WriteString("- Reviewed By: ")
		sb.WriteString(proposal.ReviewedBy)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString(proposal.Content)
	return sb.String()
}

func defaultReviewActor(actor string) string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return "launcher"
	}
	return actor
}

func parseMemoryProposalNumericID(id string) int {
	const prefix = "memory-"
	if !strings.HasPrefix(id, prefix) {
		return 0
	}
	var num int
	_, err := fmt.Sscanf(strings.TrimPrefix(id, prefix), "%d", &num)
	if err != nil || num < 0 {
		return 0
	}
	return num
}
