package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ResourceMetadata is RFC 9728 protected resource metadata.
type ResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	Scopes               []string `json:"scopes_supported,omitempty"`
}

// AuthServerMetadata is RFC 8414 authorization server metadata.
type AuthServerMetadata struct {
	Issuer                   string   `json:"issuer"`
	AuthorizationEndpoint    string   `json:"authorization_endpoint"`
	TokenEndpoint            string   `json:"token_endpoint"`
	RegistrationEndpoint     string   `json:"registration_endpoint,omitempty"`
	Scopes                   []string `json:"scopes_supported,omitempty"`
	CodeChallengeMethods     []string `json:"code_challenge_methods_supported,omitempty"`
	TokenEndpointAuthMethods []string `json:"token_endpoint_auth_methods_supported,omitempty"`
}

// Discover finds the protected resource metadata of a remote MCP URL.
func Discover(ctx context.Context, resourceURL string, client *http.Client) (*ResourceMetadata, error) {
	u, err := url.Parse(resourceURL)
	if err != nil {
		return nil, err
	}
	origin := u.Scheme + "://" + u.Host
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	var candidates []string
	if path != "" {
		candidates = append(candidates, origin+"/.well-known/oauth-protected-resource"+path)
	}
	candidates = append(candidates, origin+"/.well-known/oauth-protected-resource")
	var last error
	for _, candidate := range candidates {
		var metadata ResourceMetadata
		if err := getJSON(ctx, client, candidate, &metadata); err != nil {
			last = err
			continue
		}
		if len(metadata.AuthorizationServers) > 0 {
			return &metadata, nil
		}
		last = fmt.Errorf("%s: no authorization_servers", candidate)
	}
	if last == nil {
		last = fmt.Errorf("no protected resource metadata")
	}
	return nil, last
}

// AuthServer discovers an authorization server's metadata.
func AuthServer(ctx context.Context, issuer string, client *http.Client) (*AuthServerMetadata, error) {
	u, err := url.Parse(issuer)
	if err != nil {
		return nil, err
	}
	origin := u.Scheme + "://" + u.Host
	path := strings.TrimSuffix(u.EscapedPath(), "/")
	candidates := []string{
		origin + "/.well-known/oauth-authorization-server" + path,
		strings.TrimSuffix(issuer, "/") + "/.well-known/oauth-authorization-server",
		strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration",
	}
	var last error
	for _, candidate := range candidates {
		var metadata AuthServerMetadata
		if err := getJSON(ctx, client, candidate, &metadata); err != nil {
			last = err
			continue
		}
		if metadata.AuthorizationEndpoint != "" && metadata.TokenEndpoint != "" {
			return &metadata, nil
		}
		last = fmt.Errorf("%s: incomplete metadata", candidate)
	}
	if last == nil {
		last = fmt.Errorf("no authorization server metadata")
	}
	return nil, last
}

// Register asks an authorization server for a client, returning its ID and
// secret. redirectURI must be the exact callback used by Login.
func Register(ctx context.Context, metadata *AuthServerMetadata, redirectURI, name string, client *http.Client) (string, string, error) {
	if metadata.RegistrationEndpoint == "" {
		return "", "", fmt.Errorf("the authorization server has no registration_endpoint")
	}
	body, _ := json.Marshal(map[string]any{
		"client_name":                name,
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.RegistrationEndpoint, bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return "", "", fmt.Errorf("register: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var result struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", "", err
	}
	if result.ClientID == "" {
		return "", "", fmt.Errorf("register: no client_id")
	}
	return result.ClientID, result.ClientSecret, nil
}

func getJSON(ctx context.Context, client *http.Client, target string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", target, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
