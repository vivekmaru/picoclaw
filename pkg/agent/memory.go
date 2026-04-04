// PicoClaw - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 PicoClaw contributors

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/sipeed/picoclaw/pkg/fileutil"
)

const recentMemoryContextDays = 3

// MemoryStore manages persistent memory for the agent.
// Shared memory uses the legacy workspace/memory path.
// Scoped memory is stored under workspace/memory/teammates/... or
// workspace/memory/scopes/...
type MemoryStore struct {
	workspace  string
	memoryDir  string
	memoryFile string
	scope      string
}

// NewMemoryStore creates a new MemoryStore with the given workspace path.
// It ensures the memory directory exists.
func NewMemoryStore(workspace string) *MemoryStore {
	return NewMemoryStoreForScope(workspace, "")
}

// NewMemoryStoreForScope creates a new MemoryStore for a shared or scoped namespace.
func NewMemoryStoreForScope(workspace, scope string) *MemoryStore {
	scope = strings.TrimSpace(scope)
	memoryDir := resolveMemoryDir(workspace, scope)
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")

	// Ensure memory directory exists
	os.MkdirAll(memoryDir, 0o755)

	return &MemoryStore{
		workspace:  workspace,
		memoryDir:  memoryDir,
		memoryFile: memoryFile,
		scope:      normalizedMemoryScope(scope),
	}
}

// getTodayFile returns the path to today's daily note file (memory/YYYYMM/YYYYMMDD.md).
func (ms *MemoryStore) getTodayFile() string {
	return ms.dailyFileFor(time.Now())
}

func (ms *MemoryStore) dailyFileFor(date time.Time) string {
	today := time.Now().Format("20060102") // YYYYMMDD
	if !date.IsZero() {
		today = date.Format("20060102")
	}
	monthDir := today[:6]
	filePath := filepath.Join(ms.memoryDir, monthDir, today+".md")
	return filePath
}

func (ms *MemoryStore) LongTermPath() string {
	return ms.memoryFile
}

func (ms *MemoryStore) DailyNotesPattern() string {
	return filepath.Join(ms.memoryDir, "YYYYMM", "YYYYMMDD.md")
}

func (ms *MemoryStore) Scope() string {
	return ms.scope
}

func (ms *MemoryStore) IsShared() bool {
	return ms.scope == "" || ms.scope == "shared"
}

func (ms *MemoryStore) DisplayName() string {
	switch {
	case ms.IsShared():
		return "Shared Memory"
	case strings.HasPrefix(ms.scope, "teammate:"):
		return fmt.Sprintf("Teammate Memory (%s)", strings.TrimPrefix(ms.scope, "teammate:"))
	default:
		return fmt.Sprintf("Scoped Memory (%s)", ms.scope)
	}
}

func (ms *MemoryStore) SourcePaths(days int) []string {
	if days <= 0 {
		days = recentMemoryContextDays
	}
	paths := []string{ms.memoryFile}
	for i := range days {
		paths = append(paths, ms.dailyFileFor(time.Now().AddDate(0, 0, -i)))
	}
	return uniquePaths(paths)
}

// ReadLongTerm reads the long-term memory (MEMORY.md).
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadLongTerm() string {
	if data, err := os.ReadFile(ms.memoryFile); err == nil {
		return string(data)
	}
	return ""
}

// WriteLongTerm writes content to the long-term memory file (MEMORY.md).
func (ms *MemoryStore) WriteLongTerm(content string) error {
	// Use unified atomic write utility with explicit sync for flash storage reliability.
	// Using 0o600 (owner read/write only) for secure default permissions.
	return fileutil.WriteFileAtomic(ms.memoryFile, []byte(content), 0o600)
}

// ReadToday reads today's daily note.
// Returns empty string if the file doesn't exist.
func (ms *MemoryStore) ReadToday() string {
	todayFile := ms.getTodayFile()
	if data, err := os.ReadFile(todayFile); err == nil {
		return string(data)
	}
	return ""
}

// AppendToday appends content to today's daily note.
// If the file doesn't exist, it creates a new file with a date header.
func (ms *MemoryStore) AppendToday(content string) error {
	todayFile := ms.getTodayFile()

	// Ensure month directory exists
	monthDir := filepath.Dir(todayFile)
	if err := os.MkdirAll(monthDir, 0o755); err != nil {
		return err
	}

	var existingContent string
	if data, err := os.ReadFile(todayFile); err == nil {
		existingContent = string(data)
	}

	var newContent string
	if existingContent == "" {
		// Add header for new day
		header := fmt.Sprintf("# %s\n\n", time.Now().Format("2006-01-02"))
		newContent = header + content
	} else {
		// Append to existing content
		newContent = existingContent + "\n" + content
	}

	// Use unified atomic write utility with explicit sync for flash storage reliability.
	return fileutil.WriteFileAtomic(todayFile, []byte(newContent), 0o600)
}

// GetRecentDailyNotes returns daily notes from the last N days.
// Contents are joined with "---" separator.
func (ms *MemoryStore) GetRecentDailyNotes(days int) string {
	var sb strings.Builder
	first := true

	for i := range days {
		date := time.Now().AddDate(0, 0, -i)
		dateStr := date.Format("20060102") // YYYYMMDD
		monthDir := dateStr[:6]            // YYYYMM
		filePath := filepath.Join(ms.memoryDir, monthDir, dateStr+".md")

		if data, err := os.ReadFile(filePath); err == nil {
			if !first {
				sb.WriteString("\n\n---\n\n")
			}
			sb.Write(data)
			first = false
		}
	}

	return sb.String()
}

// GetMemoryContext returns formatted memory context for the agent prompt.
// Includes long-term memory and recent daily notes.
func (ms *MemoryStore) GetMemoryContext() string {
	longTerm := ms.ReadLongTerm()
	recentNotes := ms.GetRecentDailyNotes(recentMemoryContextDays)

	if longTerm == "" && recentNotes == "" {
		return ""
	}

	var sb strings.Builder

	if longTerm != "" {
		sb.WriteString("## Long-term Memory\n\n")
		sb.WriteString(longTerm)
	}

	if recentNotes != "" {
		if longTerm != "" {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString("## Recent Daily Notes\n\n")
		sb.WriteString(recentNotes)
	}

	return sb.String()
}

func normalizedMemoryScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" || strings.EqualFold(scope, "shared") {
		return "shared"
	}
	return scope
}

func resolveMemoryDir(workspace, scope string) string {
	scope = normalizedMemoryScope(scope)
	if scope == "shared" {
		return filepath.Join(workspace, "memory")
	}
	if strings.HasPrefix(scope, "teammate:") {
		teammateID := strings.TrimPrefix(scope, "teammate:")
		segments := sanitizeMemoryScopeSegments(teammateID)
		return filepath.Join(append([]string{workspace, "memory", "teammates"}, segments...)...)
	}
	segments := sanitizeMemoryScopeSegments(scope)
	return filepath.Join(append([]string{workspace, "memory", "scopes"}, segments...)...)
}

func sanitizeMemoryScopeSegments(scope string) []string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return []string{"default"}
	}

	rawSegments := strings.FieldsFunc(scope, func(r rune) bool {
		switch r {
		case '/', '\\', ':':
			return true
		default:
			return false
		}
	})
	if len(rawSegments) == 0 {
		rawSegments = []string{"default"}
	}

	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		sanitized := sanitizeMemoryScopeSegment(segment)
		if sanitized != "" {
			segments = append(segments, sanitized)
		}
	}
	if len(segments) == 0 {
		return []string{"default"}
	}
	return segments
}

func sanitizeMemoryScopeSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}

	var sb strings.Builder
	lastDash := false
	for _, r := range segment {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_', r == '.':
			sb.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r):
			if !lastDash && sb.Len() > 0 {
				sb.WriteByte('-')
				lastDash = true
			}
		default:
			if !lastDash && sb.Len() > 0 {
				sb.WriteByte('-')
				lastDash = true
			}
		}
	}

	result := strings.Trim(sb.String(), "-")
	if result == "" {
		return "scope"
	}
	return result
}
