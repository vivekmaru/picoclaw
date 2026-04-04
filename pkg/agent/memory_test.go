package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryStoreScopeIsolation(t *testing.T) {
	tmpDir := t.TempDir()

	shared := NewMemoryStore(tmpDir)
	reviewer := NewMemoryStoreForScope(tmpDir, "teammate:reviewer")
	custom := NewMemoryStoreForScope(tmpDir, "team/reviewer")

	if got := shared.LongTermPath(); got != filepath.Join(tmpDir, "memory", "MEMORY.md") {
		t.Fatalf("shared LongTermPath = %q", got)
	}
	if got := reviewer.LongTermPath(); got != filepath.Join(tmpDir, "memory", "teammates", "reviewer", "MEMORY.md") {
		t.Fatalf("reviewer LongTermPath = %q", got)
	}
	if got := custom.LongTermPath(); got != filepath.Join(tmpDir, "memory", "scopes", "team", "reviewer", "MEMORY.md") {
		t.Fatalf("custom LongTermPath = %q", got)
	}

	if err := shared.WriteLongTerm("shared memory"); err != nil {
		t.Fatalf("shared.WriteLongTerm: %v", err)
	}
	if err := reviewer.WriteLongTerm("reviewer memory"); err != nil {
		t.Fatalf("reviewer.WriteLongTerm: %v", err)
	}

	if got := shared.ReadLongTerm(); got != "shared memory" {
		t.Fatalf("shared.ReadLongTerm = %q", got)
	}
	if got := reviewer.ReadLongTerm(); got != "reviewer memory" {
		t.Fatalf("reviewer.ReadLongTerm = %q", got)
	}
	if got := shared.ReadLongTerm(); strings.Contains(got, "reviewer") {
		t.Fatalf("shared memory leaked teammate content: %q", got)
	}
}

func TestContextBuilderForTeammateIncludesSharedAndLocalMemory(t *testing.T) {
	tmpDir := setupWorkspace(t, map[string]string{
		"memory/MEMORY.md":                    "# Shared\nPrefer concise plans.",
		"memory/teammates/reviewer/MEMORY.md": "# Reviewer\nFocus on correctness.",
	})
	defer os.RemoveAll(tmpDir)

	cb := NewContextBuilder(tmpDir).ForTeammate(TeammateProfile{
		ID:          "reviewer",
		Name:        "Reviewer",
		MemoryScope: "teammate:reviewer",
	})

	prompt := cb.BuildSystemPrompt()
	if !strings.Contains(prompt, "# Shared Memory") {
		t.Fatalf("prompt missing shared memory section:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Prefer concise plans.") {
		t.Fatalf("prompt missing shared memory content:\n%s", prompt)
	}
	if !strings.Contains(prompt, "# Teammate Memory (Reviewer)") {
		t.Fatalf("prompt missing teammate memory section:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Focus on correctness.") {
		t.Fatalf("prompt missing teammate memory content:\n%s", prompt)
	}
	if !strings.Contains(prompt, filepath.Join(tmpDir, "memory", "teammates", "reviewer", "MEMORY.md")) {
		t.Fatalf("prompt missing teammate memory path:\n%s", prompt)
	}
}
