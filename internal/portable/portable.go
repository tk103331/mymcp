// Package portable exports and imports a profile's servers as one JSON
// bundle. Secrets are references, never values: the receiving machine needs
// the same secret names in its own vault.
package portable

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
)

// Version is the bundle format version.
const Version = 1

// Bundle is one exported profile.
type Bundle struct {
	Version   int             `json:"version"`
	Profile   store.Profile   `json:"profile"`
	Servers   []*store.Server `json:"servers"`
	Signature string          `json:"signature,omitempty"`
}

// Export reads a profile and every server attached to it.
func Export(ctx context.Context, db *store.DB, profile string) (*Bundle, error) {
	p, err := db.Profile(ctx, profile)
	if err != nil {
		return nil, err
	}
	servers, err := db.AllServersForProfile(ctx, profile)
	if err != nil {
		return nil, err
	}
	for _, server := range servers {
		for _, values := range []map[string]string{server.Env, server.Headers} {
			for key, value := range values {
				if sensitiveKey(key) && value != "" && !secrets.IsRef(value) {
					return nil, fmt.Errorf("%s: %s contains a literal secret; store it as secret:// before exporting", server.Name, key)
				}
			}
		}
		if server.OAuth != nil && server.OAuth.ClientSecret != "" && !secrets.IsRef(server.OAuth.ClientSecret) {
			return nil, fmt.Errorf("%s: the OAuth client secret is literal; store it as secret:// before exporting", server.Name)
		}
	}
	return &Bundle{Version: Version, Profile: *p, Servers: servers}, nil
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(key)
	for _, word := range []string{"token", "secret", "password", "passwd", "api_key", "apikey", "authorization", "cookie", "credential", "private_key"} {
		if strings.Contains(key, word) {
			return true
		}
	}
	return false
}

// Import merges a bundle into the database. An existing server is kept
// unless force is set. name overrides the profile name.
func Import(ctx context.Context, db *store.DB, bundle *Bundle, name string, force bool) (created, updated, skipped int, err error) {
	if bundle == nil {
		return 0, 0, 0, fmt.Errorf("empty bundle")
	}
	if bundle.Version != Version {
		return 0, 0, 0, fmt.Errorf("unsupported bundle version %d", bundle.Version)
	}
	profileName := bundle.Profile.Name
	if name != "" {
		profileName = name
	}
	if profileName == "" {
		return 0, 0, 0, fmt.Errorf("bundle has no profile name")
	}
	profile := bundle.Profile
	profile.ID, profile.Name = "", profileName
	if _, err := db.UpsertProfile(ctx, &profile); err != nil {
		return 0, 0, 0, err
	}
	for _, incoming := range bundle.Servers {
		copy := *incoming
		copy.ID = ""
		existing, getErr := db.GetServer(ctx, copy.Name)
		if getErr == nil && !force {
			skipped++
			if err := db.AttachServer(ctx, profileName, existing.Name, existing.Enabled); err != nil {
				return created, updated, skipped, err
			}
			continue
		}
		if _, err := db.UpsertServer(ctx, &copy); err != nil {
			return created, updated, skipped, err
		}
		if getErr == nil {
			updated++
		} else {
			created++
		}
		if err := db.AttachServer(ctx, profileName, copy.Name, copy.Enabled); err != nil {
			return created, updated, skipped, err
		}
	}
	return created, updated, skipped, nil
}

// Encode writes a bundle as indented JSON.
func Encode(bundle *Bundle) ([]byte, error) {
	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// Decode reads a bundle.
func Decode(raw []byte) (*Bundle, error) {
	var bundle Bundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

// Sign signs the bundle with an Ed25519 private key.
func Sign(bundle *Bundle, key ed25519.PrivateKey) error {
	bundle.Signature = ""
	payload, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	bundle.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload))
	return nil
}

// Verify checks a signed bundle with an Ed25519 public key.
func Verify(bundle *Bundle, key ed25519.PublicKey) error {
	if bundle.Signature == "" {
		return fmt.Errorf("bundle is not signed")
	}
	signature, err := base64.StdEncoding.DecodeString(bundle.Signature)
	if err != nil {
		return err
	}
	signed := *bundle
	signed.Signature = ""
	payload, err := json.Marshal(&signed)
	if err != nil {
		return err
	}
	if !ed25519.Verify(key, payload, signature) {
		return fmt.Errorf("bundle signature does not match")
	}
	return nil
}

// KeyPair is an Ed25519 signing key pair in PEM.
type KeyPair struct {
	Private []byte
	Public  []byte
}

// GenerateKeyPair makes a new signing key pair.
func GenerateKeyPair() (*KeyPair, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return nil, err
	}
	return &KeyPair{
		Private: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}),
		Public:  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}),
	}, nil
}

// ReadPrivateKey reads a PEM Ed25519 private key.
func ReadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%s: no PEM block", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	private, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s: not an Ed25519 key", path)
	}
	return private, nil
}

// ReadPublicKey reads a PEM Ed25519 public key.
func ReadPublicKey(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%s: no PEM block", path)
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	public, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s: not an Ed25519 key", path)
	}
	return public, nil
}
