package agent

import (
	"errors"
	"strings"
	"testing"
)

func TestMemoryProposalStore_ApproveWritesMemory(t *testing.T) {
	workspace := t.TempDir()
	store := NewMemoryProposalStore(workspace)

	proposal, err := store.Create(MemoryProposalRequest{
		Scope:         "teammate:reviewer",
		Target:        "long_term",
		Kind:          "task_result",
		Title:         "Patch review summary",
		Content:       "Always check migration paths before falling back.",
		SourceTaskID:  "subagent-3",
		SourceAgentID: "main",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	approved, err := store.Approve(proposal.ID, "launcher", "")
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if approved.Status != "approved" {
		t.Fatalf("approved.Status = %q, want approved", approved.Status)
	}

	mem := NewMemoryStoreForScope(workspace, "teammate:reviewer")
	content := mem.ReadLongTerm()
	if !strings.Contains(content, "Patch review summary") {
		t.Fatalf("memory content missing title: %s", content)
	}
	if !strings.Contains(content, "Always check migration paths before falling back.") {
		t.Fatalf("memory content missing proposal body: %s", content)
	}
}

func TestMemoryProposalStore_RejectKeepsMemoryUnchanged(t *testing.T) {
	workspace := t.TempDir()
	store := NewMemoryProposalStore(workspace)

	proposal, err := store.Create(MemoryProposalRequest{
		Scope:   "shared",
		Target:  "long_term",
		Content: "Do not write me",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	rejected, err := store.Reject(proposal.ID, "launcher", "Not useful")
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if rejected.Status != "rejected" {
		t.Fatalf("rejected.Status = %q, want rejected", rejected.Status)
	}

	mem := NewMemoryStore(workspace)
	if got := mem.ReadLongTerm(); got != "" {
		t.Fatalf("shared memory should remain empty after rejection, got %q", got)
	}
}

func TestMemoryProposalStore_UpdatePendingProposal(t *testing.T) {
	workspace := t.TempDir()
	store := NewMemoryProposalStore(workspace)

	proposal, err := store.Create(MemoryProposalRequest{
		Scope:   "shared",
		Target:  "long_term",
		Title:   "Original",
		Content: "Original content",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := store.Update(proposal.ID, "operator", MemoryProposalUpdate{
		Scope:   "teammate:reviewer",
		Title:   "Edited Title",
		Content: "Edited content",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Scope != "teammate:reviewer" {
		t.Fatalf("updated.Scope = %q, want teammate:reviewer", updated.Scope)
	}
	if updated.Title != "Edited Title" {
		t.Fatalf("updated.Title = %q, want Edited Title", updated.Title)
	}
	if updated.Content != "Edited content" {
		t.Fatalf("updated.Content = %q, want Edited content", updated.Content)
	}
	if updated.UpdatedBy != "operator" {
		t.Fatalf("updated.UpdatedBy = %q, want operator", updated.UpdatedBy)
	}
	if updated.UpdatedAt == 0 {
		t.Fatal("updated.UpdatedAt should be set")
	}
}

func TestMemoryProposalStore_UpdatePendingProposalRejectsBlankContent(t *testing.T) {
	workspace := t.TempDir()
	store := NewMemoryProposalStore(workspace)

	proposal, err := store.Create(MemoryProposalRequest{
		Scope:   "shared",
		Target:  "long_term",
		Content: "Original content",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = store.Update(proposal.ID, "operator", MemoryProposalUpdate{
		Scope:   "shared",
		Content: "   ",
	})
	if !errors.Is(err, errMemoryProposalInvalid) {
		t.Fatalf("Update() error = %v, want errMemoryProposalInvalid", err)
	}
}
