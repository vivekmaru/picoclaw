package api

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const defaultLauncherBootstrapTTL = 2 * time.Minute

type LauncherBootstrapManager struct {
	mu    sync.Mutex
	now   func() time.Time
	ttl   time.Duration
	codes map[string]time.Time
}

func NewLauncherBootstrapManager(ttl time.Duration) *LauncherBootstrapManager {
	if ttl <= 0 {
		ttl = defaultLauncherBootstrapTTL
	}
	return &LauncherBootstrapManager{
		now:   time.Now,
		ttl:   ttl,
		codes: make(map[string]time.Time),
	}
}

func (m *LauncherBootstrapManager) Issue() (string, error) {
	if m == nil {
		return "", nil
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := hex.EncodeToString(buf)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	m.codes[code] = m.now().Add(m.ttl)
	return code, nil
}

func (m *LauncherBootstrapManager) Consume(code string) bool {
	if m == nil || strings.TrimSpace(code) == "" {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	expiry, ok := m.codes[code]
	if !ok || m.now().After(expiry) {
		delete(m.codes, code)
		return false
	}
	delete(m.codes, code)
	return true
}

func (m *LauncherBootstrapManager) pruneLocked() {
	now := m.now()
	for code, expiry := range m.codes {
		if now.After(expiry) {
			delete(m.codes, code)
		}
	}
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
