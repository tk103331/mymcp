// Package oauth runs the authorization-code flow for one upstream MCP
// server and keeps its tokens in MyMCP's secret vault.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
)

// Token is one server's OAuth token.
type Token struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	TokenType    string    `json:"tokenType,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

// Key is the secret name holding a server's token.
func Key(server string) string { return "oauth/" + server }

// Save stores a token in the vault.
func Save(secretStore *secrets.Store, server string, token *Token) error {
	raw, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return secretStore.Set(Key(server), string(raw))
}

// Load reads a token from the vault.
func Load(secretStore *secrets.Store, server string) (*Token, error) {
	raw, err := secretStore.Get(Key(server))
	if err != nil {
		return nil, err
	}
	var token Token
	if err := json.Unmarshal([]byte(raw), &token); err != nil {
		return nil, err
	}
	return &token, nil
}

// Delete removes a token.
func Delete(secretStore *secrets.Store, server string) error {
	if !secretStore.Has(Key(server)) {
		return nil
	}
	return secretStore.Delete(Key(server))
}

// AccessToken returns a valid access token, refreshing it when it is close
// to expiry.
func AccessToken(ctx context.Context, cfg *store.OAuthConfig, server string, secretStore *secrets.Store, client *http.Client) (string, error) {
	token, err := Load(secretStore, server)
	if err != nil {
		return "", fmt.Errorf("oauth: %w (run `mymcp oauth login %s`)", err, server)
	}
	if token.AccessToken != "" && (token.Expiry.IsZero() || time.Now().Add(time.Minute).Before(token.Expiry)) {
		return token.AccessToken, nil
	}
	if token.RefreshToken == "" {
		return "", fmt.Errorf("oauth: token for %s expired and has no refresh token; run `mymcp oauth login %s`", server, server)
	}
	next, err := refresh(ctx, cfg, server, secretStore, client, token.RefreshToken)
	if err != nil {
		return "", err
	}
	return next.AccessToken, nil
}

// Login runs an authorization-code flow and stores the token. open is called
// with the authorization URL, typically to start the system browser.
func Login(ctx context.Context, cfg *store.OAuthConfig, server string, secretStore *secrets.Store, open func(string) error, out io.Writer) (*Token, error) {
	if cfg == nil || cfg.AuthorizationURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" {
		return nil, fmt.Errorf("oauth: authorization URL, token URL and client ID are required")
	}
	redirect := cfg.RedirectURI
	var listener net.Listener
	var err error
	if redirect == "" {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		redirect = "http://" + listener.Addr().String() + "/callback"
	} else {
		u, err := url.Parse(redirect)
		if err != nil {
			return nil, fmt.Errorf("oauth redirect: %w", err)
		}
		host := u.Host
		if host == "" {
			host = "127.0.0.1:0"
		}
		listener, err = net.Listen("tcp", host)
		if err != nil {
			return nil, err
		}
	}
	defer listener.Close()

	verifier, challenge, err := pkce()
	if err != nil {
		return nil, err
	}
	state, err := random(24)
	if err != nil {
		return nil, err
	}
	authorization, err := url.Parse(cfg.AuthorizationURL)
	if err != nil {
		return nil, err
	}
	query := authorization.Query()
	query.Set("response_type", "code")
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", redirect)
	query.Set("state", state)
	if len(cfg.Scopes) > 0 {
		query.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	if cfg.UsePKCE {
		query.Set("code_challenge", challenge)
		query.Set("code_challenge_method", "S256")
	}
	authorization.RawQuery = query.Encode()

	type callback struct {
		code string
		err  error
	}
	ch := make(chan callback, 1)
	path := "/"
	if u, err := url.Parse(redirect); err == nil && u.Path != "" {
		path = u.Path
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("state"); got != state {
			ch <- callback{err: fmt.Errorf("oauth: state mismatch")}
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		if e := r.URL.Query().Get("error"); e != "" {
			desc := r.URL.Query().Get("error_description")
			ch <- callback{err: fmt.Errorf("oauth: %s %s", e, desc)}
			http.Error(w, "authorization failed", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			ch <- callback{err: fmt.Errorf("oauth: no code")}
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		ch <- callback{code: code}
		fmt.Fprintln(w, "MyMCP is connected. You can close this window.")
	})}
	go srv.Serve(listener)
	defer srv.Shutdown(context.Background())

	fmt.Fprintf(out, "Open this URL to sign in:\n%s\n", authorization.String())
	if open != nil {
		if err := open(authorization.String()); err != nil {
			fmt.Fprintf(out, "could not open the browser: %v\n", err)
		}
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var code string
	select {
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	case result := <-ch:
		if result.err != nil {
			return nil, result.err
		}
		code = result.code
	}

	clientSecret, err := secretStore.Resolve(cfg.ClientSecret)
	if err != nil {
		return nil, err
	}
	token, err := exchange(ctx, http.DefaultClient, cfg, code, redirect, verifier, clientSecret)
	if err != nil {
		return nil, err
	}
	if err := Save(secretStore, server, token); err != nil {
		return nil, err
	}
	return token, nil
}

func refresh(ctx context.Context, cfg *store.OAuthConfig, server string, secretStore *secrets.Store, client *http.Client, refreshToken string) (*Token, error) {
	clientSecret, err := secretStore.Resolve(cfg.ClientSecret)
	if err != nil {
		return nil, err
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {cfg.ClientID},
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("oauth refresh: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	token, err := parseToken(raw, refreshToken)
	if err != nil {
		return nil, err
	}
	if err := Save(secretStore, server, token); err != nil {
		return nil, err
	}
	return token, nil
}

func exchange(ctx context.Context, client *http.Client, cfg *store.OAuthConfig, code, redirect, verifier, clientSecret string) (*Token, error) {
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirect},
		"client_id":    {cfg.ClientID},
	}
	if cfg.UsePKCE {
		form.Set("code_verifier", verifier)
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("oauth token: %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return parseToken(raw, "")
}

func parseToken(raw []byte, previousRefresh string) (*Token, error) {
	var response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	if response.AccessToken == "" {
		return nil, fmt.Errorf("oauth: token response has no access_token")
	}
	token := &Token{
		AccessToken:  response.AccessToken,
		RefreshToken: response.RefreshToken,
		TokenType:    response.TokenType,
	}
	if token.RefreshToken == "" {
		token.RefreshToken = previousRefresh
	}
	if response.ExpiresIn > 0 {
		token.Expiry = time.Now().Add(time.Duration(response.ExpiresIn)*time.Second - 30*time.Second)
	}
	return token, nil
}

func pkce() (string, string, error) {
	verifier, err := random(48)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

func random(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
