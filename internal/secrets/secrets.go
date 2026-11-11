// Package secrets keeps MyMCP's upstream credentials out of server
// configuration and out of the SQLite database.
//
// The first backend is an encrypted file: a 32-byte master key lives beside
// the data with mode 0600 (or comes from MYMCP_MASTER_KEY, base64), and the
// values are sealed with AES-256-GCM. A platform keychain backend can replace
// it behind the same API without touching callers.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const refPrefix = "secret://"

// Store reads and writes the local secret vault.
type Store struct {
	mu       sync.Mutex
	path     string
	keyPath  string
	key      []byte
	keyError error
}

// Open opens the vault in dir, creating the master key when needed.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	_ = os.Chmod(dir, 0o700)
	s := &Store{
		path:    filepath.Join(dir, "secrets.enc"),
		keyPath: filepath.Join(dir, "master.key"),
	}
	key, err := loadKey(s.keyPath)
	if err != nil {
		return nil, err
	}
	s.key = key
	return s, nil
}

// IsRef reports whether value names a secret.
func IsRef(value string) bool { return strings.HasPrefix(strings.TrimSpace(value), refPrefix) }

// Name returns the secret name a reference names.
func Name(ref string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ref), refPrefix))
}

// Resolve returns a literal value, or the secret's value for a reference.
func (s *Store) Resolve(value string) (string, error) {
	if !IsRef(value) {
		return value, nil
	}
	name := Name(value)
	if name == "" {
		return "", fmt.Errorf("empty secret reference")
	}
	return s.Get(name)
}

// ResolveMap resolves every value of a map.
func (s *Store) ResolveMap(values map[string]string) (map[string]string, error) {
	resolved := make(map[string]string, len(values))
	for key, value := range values {
		v, err := s.Resolve(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		resolved[key] = v
	}
	return resolved, nil
}

// Set stores a secret value.
func (s *Store) Set(name, value string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("a secret needs a name")
	}
	if strings.Contains(name, refPrefix) {
		return fmt.Errorf("a secret name cannot contain %q", refPrefix)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	values, err := s.load()
	if err != nil {
		return err
	}
	values[name] = value
	return s.save(values)
}

// Get returns a secret value.
func (s *Store) Get(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values, err := s.load()
	if err != nil {
		return "", err
	}
	value, ok := values[name]
	if !ok {
		return "", fmt.Errorf("secret %q not found", name)
	}
	return value, nil
}

// Delete removes a secret.
func (s *Store) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	values, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := values[name]; !ok {
		return fmt.Errorf("secret %q not found", name)
	}
	delete(values, name)
	return s.save(values)
}

// List returns every secret name, sorted.
func (s *Store) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values, err := s.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// Has reports whether a secret exists.
func (s *Store) Has(name string) bool {
	_, err := s.Get(name)
	return err == nil
}

func (s *Store) load() (map[string]string, error) {
	values := map[string]string{}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return values, nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("%s: truncated vault", s.path)
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.path, err)
	}
	if err := json.Unmarshal(plain, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func (s *Store) save(values map[string]string) error {
	plain, err := json.Marshal(values)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := gcm.Seal(nonce, nonce, plain, nil)
	temp := s.path + ".tmp"
	if err := os.WriteFile(temp, sealed, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temp, s.path); err != nil {
		os.Remove(temp)
		return err
	}
	return nil
}

func loadKey(path string) ([]byte, error) {
	if encoded := os.Getenv("MYMCP_MASTER_KEY"); encoded != "" {
		key, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("MYMCP_MASTER_KEY: %w", err)
		}
		if len(key) != 32 {
			return nil, fmt.Errorf("MYMCP_MASTER_KEY must decode to 32 bytes")
		}
		return key, nil
	}
	raw, err := os.ReadFile(path)
	if err == nil {
		if len(raw) != 32 {
			return nil, fmt.Errorf("%s: want a 32-byte key", path)
		}
		return raw, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}
