package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuthCredentialIsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"zero time", time.Time{}, false},
		{"future", time.Now().Add(time.Hour), false},
		{"past", time.Now().Add(-time.Hour), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AuthCredential{ExpiresAt: tt.expiresAt}
			if got := c.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthCredentialNeedsRefresh(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"zero time", time.Time{}, false},
		{"far future", time.Now().Add(time.Hour), false},
		{"within 5 min", time.Now().Add(3 * time.Minute), true},
		{"already expired", time.Now().Add(-time.Minute), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AuthCredential{ExpiresAt: tt.expiresAt}
			if got := c.NeedsRefresh(); got != tt.want {
				t.Errorf("NeedsRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cred := &AuthCredential{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		AccountID:    "acct-123",
		ExpiresAt:    time.Now().Add(time.Hour).Truncate(time.Second),
		Provider:     "openai",
		AuthMethod:   "oauth",
	}

	if err := SetCredential("openai", cred); err != nil {
		t.Fatalf("SetCredential() error: %v", err)
	}

	loaded, err := GetCredential("openai")
	if err != nil {
		t.Fatalf("GetCredential() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("GetCredential() returned nil")
	}
	if loaded.AccessToken != cred.AccessToken {
		t.Errorf("AccessToken = %q, want %q", loaded.AccessToken, cred.AccessToken)
	}
	if loaded.RefreshToken != cred.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", loaded.RefreshToken, cred.RefreshToken)
	}
	if loaded.Provider != cred.Provider {
		t.Errorf("Provider = %q, want %q", loaded.Provider, cred.Provider)
	}
}

func TestStoreFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cred := &AuthCredential{
		AccessToken: "secret-token",
		Provider:    "openai",
		AuthMethod:  "oauth",
	}
	if err := SetCredential("openai", cred); err != nil {
		t.Fatalf("SetCredential() error: %v", err)
	}

	path := filepath.Join(tmpDir, ".picoclaw", "auth.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	keyInfo, err := os.Stat(filepath.Join(tmpDir, ".picoclaw", "auth.key"))
	if err != nil {
		t.Fatalf("Stat(auth.key) error: %v", err)
	}
	if perm := keyInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("auth.key permissions = %o, want 0600", perm)
	}
}

func TestStoreMultiProvider(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	openaiCred := &AuthCredential{AccessToken: "openai-token", Provider: "openai", AuthMethod: "oauth"}
	anthropicCred := &AuthCredential{AccessToken: "anthropic-token", Provider: "anthropic", AuthMethod: "token"}

	if err := SetCredential("openai", openaiCred); err != nil {
		t.Fatalf("SetCredential(openai) error: %v", err)
	}
	if err := SetCredential("anthropic", anthropicCred); err != nil {
		t.Fatalf("SetCredential(anthropic) error: %v", err)
	}

	loaded, err := GetCredential("openai")
	if err != nil {
		t.Fatalf("GetCredential(openai) error: %v", err)
	}
	if loaded.AccessToken != "openai-token" {
		t.Errorf("openai token = %q, want %q", loaded.AccessToken, "openai-token")
	}

	loaded, err = GetCredential("anthropic")
	if err != nil {
		t.Fatalf("GetCredential(anthropic) error: %v", err)
	}
	if loaded.AccessToken != "anthropic-token" {
		t.Errorf("anthropic token = %q, want %q", loaded.AccessToken, "anthropic-token")
	}
}

func TestDeleteCredential(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cred := &AuthCredential{AccessToken: "to-delete", Provider: "openai", AuthMethod: "oauth"}
	if err := SetCredential("openai", cred); err != nil {
		t.Fatalf("SetCredential() error: %v", err)
	}

	if err := DeleteCredential("openai"); err != nil {
		t.Fatalf("DeleteCredential() error: %v", err)
	}

	loaded, err := GetCredential("openai")
	if err != nil {
		t.Fatalf("GetCredential() error: %v", err)
	}
	if loaded != nil {
		t.Error("expected nil after delete")
	}
}

func TestLoadStoreEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	store, err := LoadStore()
	if err != nil {
		t.Fatalf("LoadStore() error: %v", err)
	}
	if store == nil {
		t.Fatal("LoadStore() returned nil")
	}
	if len(store.Credentials) != 0 {
		t.Errorf("expected empty credentials, got %d", len(store.Credentials))
	}
}

func TestStoreEncryptsAtRest(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cred := &AuthCredential{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Provider:     "openai",
		AuthMethod:   "oauth",
	}
	if err := SetCredential("openai", cred); err != nil {
		t.Fatalf("SetCredential() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".picoclaw", "auth.json"))
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	text := string(data)
	if strings.Contains(text, cred.AccessToken) || strings.Contains(text, cred.RefreshToken) {
		t.Fatalf("auth.json should not contain plaintext credentials: %s", text)
	}
	if !strings.Contains(text, `"ciphertext"`) {
		t.Fatalf("auth.json should contain encrypted envelope, got: %s", text)
	}
}

func TestLoadStoreMigratesLegacyPlaintext(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	path := filepath.Join(tmpDir, ".picoclaw", "auth.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{
  "credentials": {
    "openai": {
      "access_token": "legacy-access",
      "refresh_token": "legacy-refresh",
      "provider": "openai",
      "auth_method": "oauth"
    }
  }
}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := LoadStore()
	if err != nil {
		t.Fatalf("LoadStore() error: %v", err)
	}
	cred := store.Credentials["openai"]
	if cred == nil || cred.AccessToken != "legacy-access" {
		t.Fatalf("legacy credential not loaded: %#v", cred)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "legacy-access") {
		t.Fatalf("legacy auth.json should be migrated to encrypted format: %s", string(data))
	}
}
