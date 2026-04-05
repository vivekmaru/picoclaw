package agent

import (
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
