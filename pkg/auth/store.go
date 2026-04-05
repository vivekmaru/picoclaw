package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/fileutil"
)

type AuthCredential struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Provider     string    `json:"provider"`
	AuthMethod   string    `json:"auth_method"`
	Email        string    `json:"email,omitempty"`
	ProjectID    string    `json:"project_id,omitempty"`
}

type AuthStore struct {
	Credentials map[string]*AuthCredential `json:"credentials"`
}

type authStoreEnvelope struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

const authStoreEnvelopeVersion = 1

func (c *AuthCredential) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

func (c *AuthCredential) NeedsRefresh() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}

func authFilePath() string {
	return filepath.Join(config.GetHome(), "auth.json")
}

func authKeyPath() string {
	return filepath.Join(config.GetHome(), "auth.key")
}

func LoadStore() (*AuthStore, error) {
	path := authFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AuthStore{Credentials: make(map[string]*AuthCredential)}, nil
		}
		return nil, err
	}

	store, migrated, err := decodeAuthStore(data)
	if err != nil {
		return nil, err
	}
	if migrated {
		if err := SaveStore(store); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func SaveStore(store *AuthStore) error {
	path := authFilePath()
	data, err := encodeAuthStore(store)
	if err != nil {
		return err
	}

	// Use unified atomic write utility with explicit sync for flash storage reliability.
	return fileutil.WriteFileAtomic(path, data, 0o600)
}

func GetCredential(provider string) (*AuthCredential, error) {
	store, err := LoadStore()
	if err != nil {
		return nil, err
	}
	cred, ok := store.Credentials[provider]
	if !ok {
		return nil, nil
	}
	return cred, nil
}

func SetCredential(provider string, cred *AuthCredential) error {
	store, err := LoadStore()
	if err != nil {
		return err
	}
	store.Credentials[provider] = cred
	return SaveStore(store)
}

func DeleteCredential(provider string) error {
	store, err := LoadStore()
	if err != nil {
		return err
	}
	delete(store.Credentials, provider)
	return SaveStore(store)
}

func DeleteAllCredentials() error {
	path := authFilePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func encodeAuthStore(store *AuthStore) ([]byte, error) {
	plaintext, err := json.Marshal(store)
	if err != nil {
		return nil, err
	}
	key, err := loadOrCreateAuthKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, []byte("picoclaw-auth-store-v1"))
	env := authStoreEnvelope{
		Version:    authStoreEnvelopeVersion,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}
	return json.MarshalIndent(env, "", "  ")
}

func decodeAuthStore(data []byte) (*AuthStore, bool, error) {
	var env authStoreEnvelope
	if err := json.Unmarshal(data, &env); err == nil {
		if looksLikeAuthStoreEnvelope(env) {
			if env.Version != authStoreEnvelopeVersion {
				return nil, false, fmt.Errorf(
					"unsupported auth store envelope version: got %d, want %d",
					env.Version,
					authStoreEnvelopeVersion,
				)
			}
			if env.Nonce == "" || env.Ciphertext == "" {
				return nil, false, fmt.Errorf("invalid auth store envelope")
			}
			store, err := decryptAuthStoreEnvelope(&env)
			if err != nil {
				return nil, false, err
			}
			return store, false, nil
		}
	}

	var store AuthStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, false, err
	}
	normalizeAuthStore(&store)
	return &store, true, nil
}

func looksLikeAuthStoreEnvelope(env authStoreEnvelope) bool {
	return env.Version != 0 || env.Nonce != "" || env.Ciphertext != ""
}

func decryptAuthStoreEnvelope(env *authStoreEnvelope) (*AuthStore, error) {
	key, err := loadOrCreateAuthKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode auth nonce: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode auth ciphertext: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte("picoclaw-auth-store-v1"))
	if err != nil {
		return nil, fmt.Errorf("decrypt auth store: %w", err)
	}
	var store AuthStore
	if err := json.Unmarshal(plaintext, &store); err != nil {
		return nil, err
	}
	normalizeAuthStore(&store)
	return &store, nil
}

func normalizeAuthStore(store *AuthStore) {
	if store.Credentials == nil {
		store.Credentials = make(map[string]*AuthCredential)
	}
}

func loadOrCreateAuthKey() ([]byte, error) {
	path := authKeyPath()
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("invalid auth key length: got %d, want 32", len(key))
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := fileutil.WriteFileAtomic(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}
