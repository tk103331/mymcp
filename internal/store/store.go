// Package store keeps MyMCP's gateway registry and its local usage log in a
// single SQLite file. Secrets are not stored here: they live in the encrypted
// secret store, and this database keeps only references.
package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// DB is the gateway's SQLite database.
type DB struct {
	sql *sql.DB
	now func() time.Time
}

// Server is one MCP server in the registry.
type Server struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Transport   string            `json:"transport"` // stdio | http | sse
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	URL         string            `json:"url,omitempty"`
	Env         map[string]string `json:"env,omitempty"`     // values may be secret://name
	Headers     map[string]string `json:"headers,omitempty"` // values may be secret://name
	OAuth       *OAuthConfig      `json:"oauth,omitempty"`
	Executor    string            `json:"executor,omitempty"` // "" (local) or "docker"
	Image       string            `json:"image,omitempty"`
	Network     string            `json:"network,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	Enabled     bool              `json:"enabled"`
	IdleSeconds int               `json:"idleSeconds"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// OAuthConfig is an OAuth 2.1 client for one upstream server. The auth and
// token endpoints are explicit; ClientSecret may be a secret:// reference.
type OAuthConfig struct {
	AuthorizationURL string   `json:"authorizationUrl"`
	TokenURL         string   `json:"tokenUrl"`
	ClientID         string   `json:"clientId"`
	ClientSecret     string   `json:"clientSecret,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
	RedirectURI      string   `json:"redirectUri,omitempty"`
	UsePKCE          bool     `json:"usePkce,omitempty"`
}

// Profile groups the servers one MCP endpoint exposes.
type Profile struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Call is one MCP method seen by the gateway.
type Call struct {
	ID            int64     `json:"id"`
	Time          time.Time `json:"time"`
	Profile       string    `json:"profile"`
	Server        string    `json:"server,omitempty"`
	Tool          string    `json:"tool,omitempty"`
	Method        string    `json:"method"`
	Status        string    `json:"status"` // ok | error | timeout | denied
	Error         string    `json:"error,omitempty"`
	LatencyMS     int64     `json:"latencyMs"`
	RequestBytes  int64     `json:"requestBytes,omitempty"`
	ResponseBytes int64     `json:"responseBytes,omitempty"`
}

// CallStat is an aggregate over calls.
type CallStat struct {
	Server string  `json:"server,omitempty"`
	Tool   string  `json:"tool,omitempty"`
	Status string  `json:"status"`
	Calls  int64   `json:"calls"`
	AvgMS  float64 `json:"avgMs"`
	MaxMS  int64   `json:"maxMs"`
}

// Open opens the database at path, creating its directory.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
		_ = os.Chmod(dir, 0o700)
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite writes are serialized; keeps tests deterministic.
	s := &DB{sql: db, now: time.Now}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	return s, nil
}

// Close closes the database.
func (d *DB) Close() error { return d.sql.Close() }

// SQL exposes the database for packages that grow their own tables.
func (d *DB) SQL() *sql.DB { return d.sql }

func (d *DB) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS servers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			transport TEXT NOT NULL,
			command TEXT NOT NULL DEFAULT '',
			args TEXT NOT NULL DEFAULT '[]',
			url TEXT NOT NULL DEFAULT '',
			env TEXT NOT NULL DEFAULT '{}',
			headers TEXT NOT NULL DEFAULT '{}',
			oauth TEXT NOT NULL DEFAULT '{}',
			executor TEXT NOT NULL DEFAULT '',
			image TEXT NOT NULL DEFAULT '',
			network TEXT NOT NULL DEFAULT '',
			volumes TEXT NOT NULL DEFAULT '[]',
			enabled INTEGER NOT NULL DEFAULT 1,
			idle_seconds INTEGER NOT NULL DEFAULT 600,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS profiles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS profile_servers (
			profile_id TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			server_id TEXT NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
			enabled INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (profile_id, server_id)
		)`,
		`CREATE TABLE IF NOT EXISTS calls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL,
			profile TEXT NOT NULL,
			server TEXT NOT NULL DEFAULT '',
			tool TEXT NOT NULL DEFAULT '',
			method TEXT NOT NULL,
			status TEXT NOT NULL,
			error TEXT NOT NULL DEFAULT '',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			request_bytes INTEGER NOT NULL DEFAULT 0,
			response_bytes INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS calls_ts ON calls(ts)`,
		`CREATE INDEX IF NOT EXISTS calls_server_tool ON calls(server, tool, ts)`,
		`CREATE TABLE IF NOT EXISTS gateway_keys (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			hash TEXT NOT NULL UNIQUE,
			profile TEXT NOT NULL DEFAULT '',
			rate_per_minute INTEGER NOT NULL DEFAULT 0,
			expires_at INTEGER NOT NULL DEFAULT 0,
			last_used_at INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := d.sql.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	// Existing databases predate several columns.
	columns := []struct{ name, definition string }{
		{"oauth", "TEXT NOT NULL DEFAULT '{}'"},
		{"executor", "TEXT NOT NULL DEFAULT ''"},
		{"image", "TEXT NOT NULL DEFAULT ''"},
		{"network", "TEXT NOT NULL DEFAULT ''"},
		{"volumes", "TEXT NOT NULL DEFAULT '[]'"},
	}
	for _, column := range columns {
		if _, err := d.sql.ExecContext(ctx, `ALTER TABLE servers ADD COLUMN `+column.name+` `+column.definition); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("migrate %s: %w", column.name, err)
		}
	}
	return nil
}

// EnsureDefaultProfile creates the "default" profile once.
func (d *DB) EnsureDefaultProfile(ctx context.Context) (*Profile, error) {
	profile, err := d.Profile(ctx, "default")
	if err == nil {
		return profile, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	return d.UpsertProfile(ctx, &Profile{Name: "default", Description: "Default MCP servers"})
}

// UpsertServer adds or updates a server by name and returns it.
func (d *DB) UpsertServer(ctx context.Context, server *Server) (*Server, error) {
	if server.Name == "" {
		return nil, fmt.Errorf("a server needs a name")
	}
	switch server.Executor {
	case "", "local", "docker":
	default:
		return nil, fmt.Errorf("%s: unknown executor %q", server.Name, server.Executor)
	}
	switch server.Transport {
	case "stdio":
		if server.Executor == "docker" {
			if server.Image == "" {
				return nil, fmt.Errorf("%s: a docker image is needed", server.Name)
			}
		} else if server.Command == "" {
			return nil, fmt.Errorf("%s: a command is needed", server.Name)
		}
	case "http", "sse":
		if server.Executor == "docker" {
			return nil, fmt.Errorf("%s: the Docker executor is only for stdio servers", server.Name)
		}
		if server.URL == "" {
			return nil, fmt.Errorf("%s: a URL is needed", server.Name)
		}
		if !strings.HasPrefix(server.URL, "http://") && !strings.HasPrefix(server.URL, "https://") {
			return nil, fmt.Errorf("%s: the URL must start with http:// or https://", server.Name)
		}
	default:
		return nil, fmt.Errorf("%s: unknown transport %q", server.Name, server.Transport)
	}
	now := d.now()
	if server.ID == "" {
		server.ID = uuid.NewString()
	}
	if server.IdleSeconds <= 0 {
		server.IdleSeconds = 600
	}
	server.CreatedAt = now
	server.UpdatedAt = now
	if emptyOAuth(server.OAuth) {
		server.OAuth = nil
	}
	args, _ := json.Marshal(server.Args)
	env, _ := json.Marshal(server.Env)
	headers, _ := json.Marshal(server.Headers)
	oauth, _ := json.Marshal(server.OAuth)
	volumes, _ := json.Marshal(server.Volumes)
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO servers (id, name, transport, command, args, url, env, headers, oauth,
			executor, image, network, volumes, enabled, idle_seconds, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			transport = excluded.transport, command = excluded.command, args = excluded.args,
			url = excluded.url, env = excluded.env, headers = excluded.headers,
			oauth = excluded.oauth, executor = excluded.executor, image = excluded.image,
			network = excluded.network, volumes = excluded.volumes,
			enabled = excluded.enabled, idle_seconds = excluded.idle_seconds,
			updated_at = excluded.updated_at`,
		server.ID, server.Name, server.Transport, server.Command, string(args), server.URL,
		string(env), string(headers), string(oauth), server.Executor, server.Image, server.Network,
		string(volumes), boolInt(server.Enabled), server.IdleSeconds,
		now.Unix(), now.Unix())
	if err != nil {
		return nil, err
	}
	return d.GetServer(ctx, server.Name)
}

// GetServer looks a server up by name.
func (d *DB) GetServer(ctx context.Context, name string) (*Server, error) {
	row := d.sql.QueryRowContext(ctx, `SELECT id, name, transport, command, args, url, env, headers, oauth, executor, image, network, volumes, enabled, idle_seconds, created_at, updated_at FROM servers WHERE name = ?`, name)
	return scanServer(row)
}

// ListServers lists every server, by name.
func (d *DB) ListServers(ctx context.Context) ([]*Server, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT id, name, transport, command, args, url, env, headers, oauth, executor, image, network, volumes, enabled, idle_seconds, created_at, updated_at FROM servers ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []*Server
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

// DeleteServer removes a server and its profile links.
func (d *DB) DeleteServer(ctx context.Context, name string) error {
	_, err := d.sql.ExecContext(ctx, `DELETE FROM servers WHERE name = ?`, name)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanServer(row scanner) (*Server, error) {
	var (
		server                                Server
		args, env, headers, oauthRaw, volumes string
		enabled                               int
		created, updated                      int64
	)
	if err := row.Scan(&server.ID, &server.Name, &server.Transport, &server.Command, &args,
		&server.URL, &env, &headers, &oauthRaw, &server.Executor, &server.Image, &server.Network,
		&volumes, &enabled, &server.IdleSeconds, &created, &updated); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(args), &server.Args)
	_ = json.Unmarshal([]byte(env), &server.Env)
	_ = json.Unmarshal([]byte(headers), &server.Headers)
	_ = json.Unmarshal([]byte(oauthRaw), &server.OAuth)
	_ = json.Unmarshal([]byte(volumes), &server.Volumes)
	if emptyOAuth(server.OAuth) {
		server.OAuth = nil
	}
	server.Enabled = enabled != 0
	server.CreatedAt = time.Unix(created, 0)
	server.UpdatedAt = time.Unix(updated, 0)
	return &server, nil
}

// UpsertProfile adds or updates a profile by name.
func (d *DB) UpsertProfile(ctx context.Context, profile *Profile) (*Profile, error) {
	if profile.Name == "" {
		return nil, fmt.Errorf("a profile needs a name")
	}
	if profile.Name == "." || profile.Name == ".." || strings.ContainsAny(profile.Name, `/\?#`) ||
		strings.IndexFunc(profile.Name, unicode.IsSpace) >= 0 {
		return nil, fmt.Errorf("profile name %q contains a character that cannot appear in a URL", profile.Name)
	}
	now := d.now()
	if profile.ID == "" {
		profile.ID = uuid.NewString()
	}
	profile.CreatedAt = now
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO profiles (id, name, description, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET description = excluded.description`,
		profile.ID, profile.Name, profile.Description, now.Unix())
	if err != nil {
		return nil, err
	}
	return d.Profile(ctx, profile.Name)
}

// Profile looks a profile up by name.
func (d *DB) Profile(ctx context.Context, name string) (*Profile, error) {
	row := d.sql.QueryRowContext(ctx, `SELECT id, name, description, created_at FROM profiles WHERE name = ?`, name)
	var p Profile
	var created int64
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &created); err != nil {
		return nil, err
	}
	p.CreatedAt = time.Unix(created, 0)
	return &p, nil
}

// ListProfiles lists every profile.
func (d *DB) ListProfiles(ctx context.Context) ([]*Profile, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT id, name, description, created_at FROM profiles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var profiles []*Profile
	for rows.Next() {
		var p Profile
		var created int64
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &created); err != nil {
			return nil, err
		}
		p.CreatedAt = time.Unix(created, 0)
		profiles = append(profiles, &p)
	}
	return profiles, rows.Err()
}

// AttachServer puts a server in a profile, replacing its enabled state.
func (d *DB) AttachServer(ctx context.Context, profile, server string, enabled bool) error {
	p, err := d.Profile(ctx, profile)
	if err != nil {
		return fmt.Errorf("profile %s: %w", profile, err)
	}
	s, err := d.GetServer(ctx, server)
	if err != nil {
		return fmt.Errorf("server %s: %w", server, err)
	}
	_, err = d.sql.ExecContext(ctx, `
		INSERT INTO profile_servers (profile_id, server_id, enabled) VALUES (?, ?, ?)
		ON CONFLICT(profile_id, server_id) DO UPDATE SET enabled = excluded.enabled`,
		p.ID, s.ID, boolInt(enabled))
	return err
}

// DetachServer removes a server from a profile.
func (d *DB) DetachServer(ctx context.Context, profile, server string) error {
	p, err := d.Profile(ctx, profile)
	if err != nil {
		return err
	}
	s, err := d.GetServer(ctx, server)
	if err != nil {
		return err
	}
	_, err = d.sql.ExecContext(ctx, `DELETE FROM profile_servers WHERE profile_id = ? AND server_id = ?`, p.ID, s.ID)
	return err
}

// ServersForProfile lists the enabled servers of a profile.
func (d *DB) ServersForProfile(ctx context.Context, profile string) ([]*Server, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT s.id, s.name, s.transport, s.command, s.args, s.url, s.env, s.headers, s.oauth,
		       s.executor, s.image, s.network, s.volumes,
		       s.enabled, s.idle_seconds, s.created_at, s.updated_at
		FROM servers s
		JOIN profile_servers ps ON ps.server_id = s.id
		JOIN profiles p ON p.id = ps.profile_id
		WHERE p.name = ? AND ps.enabled = 1 AND s.enabled = 1
		ORDER BY s.name`, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []*Server
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}

// RecordCall appends one usage row.
func (d *DB) RecordCall(ctx context.Context, call *Call) error {
	if call.Time.IsZero() {
		call.Time = d.now()
	}
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO calls (ts, profile, server, tool, method, status, error, latency_ms, request_bytes, response_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		call.Time.Unix(), call.Profile, call.Server, call.Tool, call.Method, call.Status,
		call.Error, call.LatencyMS, call.RequestBytes, call.ResponseBytes)
	return err
}

// RecentCalls returns the newest calls first.
func (d *DB) RecentCalls(ctx context.Context, limit int) ([]*Call, error) {
	return d.RecentCallsSince(ctx, time.Time{}, limit)
}

// RecentCallsSince returns the newest calls at or after since, first.
func (d *DB) RecentCallsSince(ctx context.Context, since time.Time, limit int) ([]*Call, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, ts, profile, server, tool, method, status, error, latency_ms, request_bytes, response_bytes
		FROM calls WHERE ts >= ? ORDER BY id DESC LIMIT ?`, since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var calls []*Call
	for rows.Next() {
		var c Call
		var ts int64
		if err := rows.Scan(&c.ID, &ts, &c.Profile, &c.Server, &c.Tool, &c.Method, &c.Status,
			&c.Error, &c.LatencyMS, &c.RequestBytes, &c.ResponseBytes); err != nil {
			return nil, err
		}
		c.Time = time.Unix(ts, 0)
		calls = append(calls, &c)
	}
	return calls, rows.Err()
}

// CallStats aggregates calls since a time by server, tool and status.
func (d *DB) CallStats(ctx context.Context, since time.Time) ([]*CallStat, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT server, tool, status, count(*), avg(latency_ms), max(latency_ms)
		FROM calls WHERE ts >= ?
		GROUP BY server, tool, status
		ORDER BY count(*) DESC`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stats []*CallStat
	for rows.Next() {
		var s CallStat
		if err := rows.Scan(&s.Server, &s.Tool, &s.Status, &s.Calls, &s.AvgMS, &s.MaxMS); err != nil {
			return nil, err
		}
		stats = append(stats, &s)
	}
	return stats, rows.Err()
}

func emptyOAuth(cfg *OAuthConfig) bool {
	if cfg == nil {
		return true
	}
	return cfg.AuthorizationURL == "" && cfg.TokenURL == "" && cfg.ClientID == "" &&
		cfg.ClientSecret == "" && len(cfg.Scopes) == 0 && cfg.RedirectURI == ""
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// UsageSummary is an aggregate for the usage dashboard.
type UsageSummary struct {
	Since    time.Time   `json:"since"`
	Calls    int64       `json:"calls"`
	Errors   int64       `json:"errors"`
	Timeouts int64       `json:"timeouts"`
	AvgMS    float64     `json:"avgMs"`
	P95MS    float64     `json:"p95Ms"`
	Servers  int64       `json:"servers"`
	Tools    int64       `json:"tools"`
	Top      []*CallStat `json:"top,omitempty"`
}

// UsagePoint is one bucket of the usage series.
type UsagePoint struct {
	Bucket string `json:"bucket"`
	Calls  int64  `json:"calls"`
	Errors int64  `json:"errors"`
}

// UsageSummary aggregates calls since a time.
func (d *DB) UsageSummary(ctx context.Context, since time.Time) (*UsageSummary, error) {
	s := &UsageSummary{Since: since}
	err := d.sql.QueryRowContext(ctx, `
		SELECT count(*),
		       coalesce(sum(CASE WHEN status != 'ok' THEN 1 ELSE 0 END), 0),
		       coalesce(sum(CASE WHEN status = 'timeout' THEN 1 ELSE 0 END), 0),
		       coalesce(avg(latency_ms), 0),
		       count(DISTINCT server), count(DISTINCT tool)
		FROM calls WHERE ts >= ?`, since.Unix()).Scan(
		&s.Calls, &s.Errors, &s.Timeouts, &s.AvgMS, &s.Servers, &s.Tools)
	if err != nil {
		return nil, err
	}
	if s.Calls > 0 {
		row := d.sql.QueryRowContext(ctx, `
			SELECT latency_ms FROM calls WHERE ts >= ?
			ORDER BY latency_ms LIMIT 1 OFFSET (
				SELECT max(0, count(*) * 95 / 100 - 1) FROM calls WHERE ts >= ?)`,
			since.Unix(), since.Unix())
		var p95 int64
		if err := row.Scan(&p95); err == nil {
			s.P95MS = float64(p95)
		}
	}
	top, err := d.CallStats(ctx, since)
	if err != nil {
		return nil, err
	}
	if len(top) > 10 {
		top = top[:10]
	}
	s.Top = top
	return s, nil
}

// UsageSeries buckets calls by hour since a time.
func (d *DB) UsageSeries(ctx context.Context, since time.Time) ([]*UsagePoint, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT strftime('%Y-%m-%dT%H:00', ts, 'unixepoch', 'localtime') AS bucket,
		       count(*), coalesce(sum(CASE WHEN status != 'ok' THEN 1 ELSE 0 END), 0)
		FROM calls WHERE ts >= ?
		GROUP BY bucket ORDER BY bucket`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := []*UsagePoint{}
	for rows.Next() {
		var p UsagePoint
		if err := rows.Scan(&p.Bucket, &p.Calls, &p.Errors); err != nil {
			return nil, err
		}
		points = append(points, &p)
	}
	return points, rows.Err()
}

// GatewayKey is a client key for the local gateway. Token is only returned
// when the key is created; the database keeps its hash.
type GatewayKey struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Profile    string    `json:"profile,omitempty"`
	RatePerMin int       `json:"ratePerMinute,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt,omitempty"`
	LastUsedAt time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CreateKey creates a gateway key and returns its token once.
func (d *DB) CreateKey(ctx context.Context, name, profile string, ratePerMin int, expires time.Time) (string, *GatewayKey, error) {
	if strings.TrimSpace(name) == "" {
		return "", nil, fmt.Errorf("a key needs a name")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := "mymcp_" + base64.RawURLEncoding.EncodeToString(raw)
	key := &GatewayKey{
		ID: uuid.NewString(), Name: name, Profile: profile,
		RatePerMin: ratePerMin, ExpiresAt: expires, CreatedAt: d.now(),
	}
	if !expires.IsZero() {
		key.ExpiresAt = expires
	}
	var expiresUnix int64
	if !key.ExpiresAt.IsZero() {
		expiresUnix = key.ExpiresAt.Unix()
	}
	_, err := d.sql.ExecContext(ctx, `
		INSERT INTO gateway_keys (id, name, hash, profile, rate_per_minute, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		key.ID, key.Name, hashToken(token), key.Profile, key.RatePerMin, expiresUnix, key.CreatedAt.Unix())
	if err != nil {
		return "", nil, err
	}
	return token, key, nil
}

// ListKeys lists gateway keys without their hashes.
func (d *DB) ListKeys(ctx context.Context) ([]*GatewayKey, error) {
	rows, err := d.sql.QueryContext(ctx, `SELECT id, name, profile, rate_per_minute, expires_at, last_used_at, created_at FROM gateway_keys ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []*GatewayKey
	for rows.Next() {
		var key GatewayKey
		var expires, lastUsed, created int64
		if err := rows.Scan(&key.ID, &key.Name, &key.Profile, &key.RatePerMin, &expires, &lastUsed, &created); err != nil {
			return nil, err
		}
		if expires > 0 {
			key.ExpiresAt = time.Unix(expires, 0)
		}
		if lastUsed > 0 {
			key.LastUsedAt = time.Unix(lastUsed, 0)
		}
		key.CreatedAt = time.Unix(created, 0)
		keys = append(keys, &key)
	}
	return keys, rows.Err()
}

// RevokeKey removes a key by name.
func (d *DB) RevokeKey(ctx context.Context, name string) error {
	result, err := d.sql.ExecContext(ctx, `DELETE FROM gateway_keys WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("key %q not found", name)
	}
	return nil
}

// ValidateKey returns the key a token names.
func (d *DB) ValidateKey(ctx context.Context, token string) (*GatewayKey, error) {
	hash := hashToken(token)
	row := d.sql.QueryRowContext(ctx, `SELECT id, name, profile, rate_per_minute, expires_at, last_used_at, created_at FROM gateway_keys WHERE hash = ?`, hash)
	var key GatewayKey
	var expires, lastUsed, created int64
	if err := row.Scan(&key.ID, &key.Name, &key.Profile, &key.RatePerMin, &expires, &lastUsed, &created); err != nil {
		return nil, err
	}
	if expires > 0 {
		key.ExpiresAt = time.Unix(expires, 0)
		if time.Now().After(key.ExpiresAt) {
			return nil, fmt.Errorf("key %q expired", key.Name)
		}
	}
	key.LastUsedAt = time.Now()
	key.CreatedAt = time.Unix(created, 0)
	_, _ = d.sql.ExecContext(ctx, `UPDATE gateway_keys SET last_used_at = ? WHERE id = ?`, key.LastUsedAt.Unix(), key.ID)
	return &key, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SetServerOAuth replaces a server's OAuth configuration.
func (d *DB) SetServerOAuth(ctx context.Context, name string, cfg *OAuthConfig) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	result, err := d.sql.ExecContext(ctx, `UPDATE servers SET oauth = ?, updated_at = ? WHERE name = ?`,
		string(raw), d.now().Unix(), name)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("server %q not found", name)
	}
	return nil
}

// AllServersForProfile lists every server attached to a profile, enabled or
// not, for export.
func (d *DB) AllServersForProfile(ctx context.Context, profile string) ([]*Server, error) {
	rows, err := d.sql.QueryContext(ctx, `
		SELECT s.id, s.name, s.transport, s.command, s.args, s.url, s.env, s.headers, s.oauth,
		       s.executor, s.image, s.network, s.volumes,
		       s.enabled, s.idle_seconds, s.created_at, s.updated_at
		FROM servers s
		JOIN profile_servers ps ON ps.server_id = s.id
		JOIN profiles p ON p.id = ps.profile_id
		WHERE p.name = ?
		ORDER BY s.name`, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []*Server
	for rows.Next() {
		server, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, rows.Err()
}
