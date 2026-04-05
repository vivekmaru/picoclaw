package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

var writeMu sync.Mutex

type Entry struct {
	Timestamp time.Time      `json:"timestamp"`
	Event     string         `json:"event"`
	Tool      string         `json:"tool,omitempty"`
	Decision  string         `json:"decision,omitempty"`
	Reason    string         `json:"reason,omitempty"`
	Channel   string         `json:"channel,omitempty"`
	ChatID    string         `json:"chat_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type Logger struct {
	enabled bool
	path    string
}

func NewLogger(trust config.TrustConfig) *Logger {
	return &Logger{
		enabled: trust.Audit.Enabled,
		path:    trust.EffectiveAuditPath(),
	}
}

func (l *Logger) Write(entry Entry) error {
	if l == nil || !l.enabled {
		return nil
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	writeMu.Lock()
	defer writeMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
