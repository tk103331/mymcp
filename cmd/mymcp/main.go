// Command mymcp is the local MCP gateway daemon and its small CLI.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/tk103331/mymcp/internal/agent"
	"github.com/tk103331/mymcp/internal/configsync"
	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/gateway"
	"github.com/tk103331/mymcp/internal/importer"
	"github.com/tk103331/mymcp/internal/oauth"
	"github.com/tk103331/mymcp/internal/plugin"
	"github.com/tk103331/mymcp/internal/portable"
	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
)

func main() {
	log.SetFlags(log.Ltime)
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:])
	case "add":
		err = add(os.Args[2:])
	case "list":
		err = list(os.Args[2:])
	case "remove":
		err = remove(os.Args[2:])
	case "profile":
		err = profile(os.Args[2:])
	case "secret":
		err = secret(os.Args[2:])
	case "client":
		err = client(os.Args[2:])
	case "usage":
		err = usage(os.Args[2:])
	case "import":
		err = importServers(os.Args[2:])
	case "plugin":
		err = pluginCommand(os.Args[2:])
	case "oauth":
		err = oauthCommand(os.Args[2:])
	case "key":
		err = key(os.Args[2:])
	case "doctor":
		err = doctor(os.Args[2:])
	case "version":
		fmt.Println("mymcp", gateway.Version)
	default:
		printUsage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mymcp:", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `mymcp - local MCP gateway

  mymcp serve [--addr 127.0.0.1:8422]
  mymcp add NAME (--url URL | -- COMMAND [ARGS...])
  mymcp list
  mymcp remove NAME
  mymcp profile list|add|attach|detach|export|import|keygen
  mymcp secret set NAME [--value VALUE] | list | remove NAME
  mymcp client list | plan|apply|detach|restore ID [--url URL]
  mymcp usage [--limit N] [--stats] [--since 24h]
  mymcp key create NAME [--profile P] [--rate N] [--expires 30d] | list | revoke NAME
  mymcp import [--profile P] [--client ID] [--only NAMES] [--dry-run]
  mymcp plugin list | install DIR | remove ID
  mymcp oauth discover SERVER | login SERVER | status SERVER | logout SERVER
  mymcp doctor

Data lives in $MYMCP_HOME, default ~/.mymcp.`)
}

func home() string {
	if dir := os.Getenv("MYMCP_HOME"); dir != "" {
		return dir
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		return ".mymcp"
	}
	return filepath.Join(dir, ".mymcp")
}

func open() (*store.DB, *secrets.Store, error) {
	dir := home()
	db, err := store.Open(filepath.Join(dir, "mymcp.db"))
	if err != nil {
		return nil, nil, err
	}
	secretStore, err := secrets.Open(dir)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, secretStore, nil
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", defaultGatewayAddr(), "listen address")
	allowRemote := fs.Bool("allow-remote", false, "listen beyond loopback (remote clients still need a gateway key)")
	toolSearch := fs.Bool("tool-search", false, "expose find_tools/call_tool once a profile has many tools")
	toolThreshold := fs.Int("tool-threshold", 10, "tools above which --tool-search applies")
	requireKey := fs.Bool("require-key", false, "require a gateway key even on loopback")
	tlsCert := fs.String("tls-cert", "", "TLS certificate for HTTPS")
	tlsKey := fs.String("tls-key", "", "TLS private key for HTTPS")
	clientCA := fs.String("client-ca", "", "require client certificates signed by this CA")
	if err := fs.Parse(args); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return err
	}
	if !*allowRemote && !isLoopback(host) {
		return fmt.Errorf("refusing to listen on %s without --allow-remote", host)
	}
	db, secretStore, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	scheme := "http"
	if *tlsCert != "" || *tlsKey != "" {
		if *tlsCert == "" || *tlsKey == "" {
			return fmt.Errorf("both --tls-cert and --tls-key are needed")
		}
		scheme = "https"
	}
	if *clientCA != "" && scheme != "https" {
		return fmt.Errorf("--client-ca needs --tls-cert and --tls-key")
	}
	manager, err := gateway.NewManager(ctx, db, secretStore, gateway.ManagerOptions{
		BaseURL: scheme + "://" + *addr, ToolSearch: *toolSearch, ToolSearchThreshold: *toolThreshold,
	})
	if err != nil {
		return err
	}
	defer manager.Close()

	view := manager.Handler()
	if *requireKey {
		view = gateway.NewAuth(db, true).Wrap(view)
	} else {
		view = gateway.NewAuth(db, false).Wrap(view)
	}
	server := &http.Server{
		Addr:              *addr,
		Handler:           view,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if scheme == "https" {
		certificate, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		if err != nil {
			return err
		}
		server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
		if *clientCA != "" {
			pem, err := os.ReadFile(*clientCA)
			if err != nil {
				return err
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return fmt.Errorf("no certificates in %s", *clientCA)
			}
			server.TLSConfig.ClientCAs = pool
			server.TLSConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}
	log.Printf("MyMCP %s listening on %s://%s", gateway.Version, scheme, *addr)
	log.Printf("health: %s://%s/healthz", scheme, *addr)
	log.Printf("usage: %s://%s/dashboard", scheme, *addr)
	for name, status := range manager.Statuses() {
		log.Printf("profile %s: %d servers, %d tools, streamable %s", name, status.Servers, status.Tools, status.Endpoints.Streamable)
		for serverName, why := range status.Errors {
			log.Printf("profile %s: %s failed: %s", name, serverName, why)
		}
	}

	errCh := make(chan error, 1)
	go func() {
		if scheme == "https" {
			errCh <- server.ListenAndServeTLS("", "")
		} else {
			errCh <- server.ListenAndServe()
		}
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-signals:
		log.Printf("received %s, stopping", sig)
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func add(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	url := fs.String("url", "", "remote MCP URL")
	transport := fs.String("transport", "", "stdio, http or sse")
	profileName := fs.String("profile", "default", "profile to attach the server to")
	disabled := fs.Bool("disabled", false, "add the server disabled")
	idle := fs.Int("idle", 600, "idle seconds before a stdio server can stop")
	var env, headers, scopes, volumes stringList
	executor := fs.String("executor", "", "stdio executor: local or docker")
	image := fs.String("image", "", "docker image for --executor docker")
	network := fs.String("network", "", "docker network (default bridge)")
	oauthAuth := fs.String("oauth-authorization-url", "", "OAuth authorization endpoint")
	oauthToken := fs.String("oauth-token-url", "", "OAuth token endpoint")
	oauthClient := fs.String("oauth-client-id", "", "OAuth client ID")
	oauthSecret := fs.String("oauth-client-secret", "", "OAuth client secret or secret://name")
	oauthRedirect := fs.String("oauth-redirect", "", "OAuth redirect URI (default: loopback)")
	oauthPKCE := fs.Bool("oauth-pkce", true, "use PKCE")
	fs.Var(&env, "env", "KEY=VALUE or KEY=secret://name, repeatable")
	fs.Var(&headers, "header", "Header=value or Header=secret://name, repeatable")
	fs.Var(&scopes, "oauth-scope", "OAuth scope, repeatable")
	fs.Var(&volumes, "volume", "docker volume, repeatable")
	// A name before the flags is the common form; flags may also come first.
	name := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, args = args[0], args[1:]
	}
	_ = fs.Parse(args)
	if name == "" && fs.NArg() > 0 {
		name = fs.Arg(0)
		args = fs.Args()[1:]
	} else {
		args = fs.Args()
	}
	if name == "" {
		return fmt.Errorf("usage: mymcp add NAME [--url URL | -- COMMAND [ARGS...]]")
	}
	command := ""
	var commandArgs []string
	if len(args) > 0 {
		command = args[0]
		commandArgs = args[1:]
	}
	if *url == "" && command == "" {
		return fmt.Errorf("a URL or a command is needed")
	}
	kind := *transport
	if kind == "" {
		if *url != "" {
			kind = "http"
		} else {
			kind = "stdio"
		}
	}
	envMap, err := parsePairs(env)
	if err != nil {
		return err
	}
	headerMap, err := parsePairs(headers)
	if err != nil {
		return err
	}
	db, _, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.EnsureDefaultProfile(ctx); err != nil {
		return err
	}
	if _, err := db.Profile(ctx, *profileName); err != nil {
		return fmt.Errorf("profile %q not found; run `mymcp profile add %s` first", *profileName, *profileName)
	}
	var oauthConfig *store.OAuthConfig
	if *oauthAuth != "" || *oauthToken != "" || *oauthClient != "" || len(scopes) > 0 {
		if *oauthAuth == "" || *oauthToken == "" || *oauthClient == "" {
			return fmt.Errorf("OAuth needs --oauth-authorization-url, --oauth-token-url and --oauth-client-id")
		}
		oauthConfig = &store.OAuthConfig{
			AuthorizationURL: *oauthAuth, TokenURL: *oauthToken, ClientID: *oauthClient,
			ClientSecret: *oauthSecret, Scopes: scopes, RedirectURI: *oauthRedirect, UsePKCE: *oauthPKCE,
		}
	}
	server, err := db.UpsertServer(ctx, &store.Server{
		Name:        name,
		Transport:   kind,
		Command:     command,
		Args:        commandArgs,
		URL:         *url,
		Env:         envMap,
		Headers:     headerMap,
		OAuth:       oauthConfig,
		Executor:    *executor,
		Image:       *image,
		Network:     *network,
		Volumes:     volumes,
		Enabled:     !*disabled,
		IdleSeconds: *idle,
	})
	if err != nil {
		return err
	}
	if err := db.AttachServer(ctx, *profileName, name, !*disabled); err != nil {
		return err
	}
	fmt.Printf("added %s to profile %s\n", server.Name, *profileName)
	return nil
}

// defaultGatewayAddr follows the desktop app's configured endpoint so the
// CLI writes the same URL clients should use.
func defaultGatewayAddr() string {
	raw, err := os.ReadFile(filepath.Join(home(), "settings.json"))
	if err == nil {
		var settings data.Settings
		if json.Unmarshal(raw, &settings) == nil && settings.GatewayAddr != "" {
			return settings.GatewayAddr
		}
	}
	return "127.0.0.1:8422"
}

func parsePairs(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("want KEY=VALUE, got %q", value)
		}
		out[key] = val
	}
	return out, nil
}

func list(args []string) error {
	db, _, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	servers, err := db.ListServers(ctx)
	if err != nil {
		return err
	}
	profiles, err := db.ListProfiles(ctx)
	if err != nil {
		return err
	}
	fmt.Println("Servers:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, server := range servers {
		status := "on"
		if !server.Enabled {
			status = "off"
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", server.Name, server.Transport, status, endpointOf(server))
	}
	w.Flush()
	fmt.Println("Profiles:")
	for _, profile := range profiles {
		servers, _ := db.ServersForProfile(ctx, profile.Name)
		fmt.Printf("  %s\t%d servers\t%s\n", profile.Name, len(servers), profile.Description)
	}
	return nil
}

func endpointOf(server *store.Server) string {
	if server.Transport == "stdio" {
		return strings.TrimSpace(server.Command + " " + strings.Join(server.Args, " "))
	}
	return server.URL
}

func remove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: mymcp remove NAME")
	}
	db, _, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.DeleteServer(context.Background(), fs.Arg(0))
}

func profile(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mymcp profile list|add|attach|detach")
	}
	db, _, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	switch args[0] {
	case "list":
		profiles, err := db.ListProfiles(ctx)
		if err != nil {
			return err
		}
		for _, profile := range profiles {
			fmt.Printf("%s\t%s\n", profile.Name, profile.Description)
		}
		return nil
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("usage: mymcp profile add NAME [DESCRIPTION]")
		}
		description := ""
		if len(args) > 2 {
			description = strings.Join(args[2:], " ")
		}
		_, err := db.UpsertProfile(ctx, &store.Profile{Name: args[1], Description: description})
		return err
	case "attach", "detach":
		if len(args) != 3 {
			return fmt.Errorf("usage: mymcp profile %s PROFILE SERVER", args[0])
		}
		if args[0] == "attach" {
			return db.AttachServer(ctx, args[1], args[2], true)
		}
		return db.DetachServer(ctx, args[1], args[2])
	case "export":
		fs := flag.NewFlagSet("profile export", flag.ExitOnError)
		output := fs.String("output", "", "file to write (default stdout)")
		signKey := fs.String("sign-key", "", "Ed25519 private key PEM to sign the bundle")
		rest := args[1:]
		profileName := ""
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			profileName, rest = rest[0], rest[1:]
		}
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if profileName == "" && fs.NArg() > 0 {
			profileName = fs.Arg(0)
		}
		if profileName == "" {
			return fmt.Errorf("usage: mymcp profile export NAME [--output FILE]")
		}
		bundle, err := portable.Export(ctx, db, profileName)
		if err != nil {
			return err
		}
		if *signKey != "" {
			key, err := portable.ReadPrivateKey(*signKey)
			if err != nil {
				return err
			}
			if err := portable.Sign(bundle, key); err != nil {
				return err
			}
		}
		raw, err := portable.Encode(bundle)
		if err != nil {
			return err
		}
		if *output == "" {
			_, err = os.Stdout.Write(raw)
			return err
		}
		return os.WriteFile(*output, raw, 0o600)
	case "import":
		fs := flag.NewFlagSet("profile import", flag.ExitOnError)
		name := fs.String("name", "", "profile name in this database (default the bundle's)")
		force := fs.Bool("force", false, "overwrite servers that already exist")
		verifyKey := fs.String("verify-key", "", "Ed25519 public key PEM to verify the bundle")
		requireSignature := fs.Bool("require-signature", false, "refuse an unsigned bundle")
		rest := args[1:]
		file := ""
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			file, rest = rest[0], rest[1:]
		}
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if file == "" && fs.NArg() > 0 {
			file = fs.Arg(0)
		}
		if file == "" {
			return fmt.Errorf("usage: mymcp profile import FILE [--name NAME] [--force]")
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		bundle, err := portable.Decode(raw)
		if err != nil {
			return err
		}
		if *verifyKey != "" {
			key, err := portable.ReadPublicKey(*verifyKey)
			if err != nil {
				return err
			}
			if err := portable.Verify(bundle, key); err != nil {
				return err
			}
		} else if bundle.Signature != "" {
			return fmt.Errorf("bundle is signed; pass --verify-key")
		} else if *requireSignature {
			return fmt.Errorf("bundle is not signed")
		}
		created, updated, skipped, err := portable.Import(ctx, db, bundle, *name, *force)
		if err != nil {
			return err
		}
		fmt.Printf("profile %s: %d created, %d updated, %d kept\n", bundle.Profile.Name, created, updated, skipped)
		fmt.Println("note: secret:// references need the same secret names in this vault")
		return nil
	case "keygen":
		fs := flag.NewFlagSet("profile keygen", flag.ExitOnError)
		privatePath := fs.String("private", "bundle.key", "private key output")
		publicPath := fs.String("public", "bundle.pub", "public key output")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		pair, err := portable.GenerateKeyPair()
		if err != nil {
			return err
		}
		if err := os.WriteFile(*privatePath, pair.Private, 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(*publicPath, pair.Public, 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s and %s\n", *privatePath, *publicPath)
		return nil
	default:
		return fmt.Errorf("unknown profile command %q", args[0])
	}
}

func secret(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mymcp secret set|list|remove")
	}
	_, secretStore, err := open()
	if err != nil {
		return err
	}
	switch args[0] {
	case "set":
		fs := flag.NewFlagSet("secret set", flag.ExitOnError)
		value := fs.String("value", "", "value (otherwise read stdin)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: mymcp secret set NAME [--value VALUE]")
		}
		secretValue := *value
		if secretValue == "" {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			secretValue = strings.TrimRight(string(raw), "\r\n")
		}
		if secretValue == "" {
			return fmt.Errorf("an empty secret is not stored")
		}
		if err := secretStore.Set(fs.Arg(0), secretValue); err != nil {
			return err
		}
		fmt.Println("stored", fs.Arg(0))
		return nil
	case "list":
		names, err := secretStore.List()
		if err != nil {
			return err
		}
		for _, name := range names {
			fmt.Println(name)
		}
		return nil
	case "remove", "rm":
		if len(args) != 2 {
			return fmt.Errorf("usage: mymcp secret remove NAME")
		}
		return secretStore.Delete(args[1])
	default:
		return fmt.Errorf("unknown secret command %q", args[0])
	}
}

func usage(args []string) error {
	fs := flag.NewFlagSet("usage", flag.ExitOnError)
	limit := fs.Int("limit", 20, "recent calls")
	stats := fs.Bool("stats", false, "aggregate instead of listing calls")
	since := fs.Duration("since", 24*time.Hour, "window for --stats")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, _, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	if *stats {
		rows, err := db.CallStats(ctx, time.Now().Add(-*since))
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SERVER\tTOOL\tSTATUS\tCALLS\tAVG_MS\tMAX_MS")
		for _, row := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%.1f\t%d\n", row.Server, row.Tool, row.Status, row.Calls, row.AvgMS, row.MaxMS)
		}
		return w.Flush()
	}
	calls, err := db.RecentCalls(ctx, *limit)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tPROFILE\tSERVER\tTOOL\tSTATUS\tMS")
	for _, call := range calls {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n", call.Time.Format("15:04:05"), call.Profile, call.Server, call.Tool, call.Status, call.LatencyMS)
	}
	return w.Flush()
}

func client(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mymcp client list|plan|apply|detach|restore")
	}
	name := "mcp-pipe"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	helper := filepath.Join(home(), "bin", name)
	if info, err := os.Stat(helper); err == nil && info.Mode().IsRegular() {
		configsync.SetBridgeCommand(helper)
	}
	manager, err := configsync.New(home())
	if err != nil {
		return err
	}
	switch args[0] {
	case "list":
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CLIENT\tLABEL\tFILE\tPRESENT\tMANAGED\tDRIFTED")
		for _, status := range manager.Statuses() {
			fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%t\t%t\n", status.Client, status.Label, status.File, status.Present, status.Managed, status.Drifted)
		}
		return w.Flush()
	case "plan", "apply", "detach", "restore":
		fs := flag.NewFlagSet("client "+args[0], flag.ExitOnError)
		url := fs.String("url", "", "gateway URL, e.g. http://127.0.0.1:8422/mcp/default")
		addr := fs.String("addr", defaultGatewayAddr(), "gateway address used when --url is empty")
		profile := fs.String("profile", "default", "profile used when --url is empty")
		entry := fs.String("entry", "mymcp", "client entry name")
		force := fs.Bool("force", false, "overwrite or remove an entry changed by the user")
		direct := fs.Bool("direct", false, "write every profile server directly instead of one router entry")
		rest := args[1:]
		id := ""
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			id, rest = rest[0], rest[1:]
		}
		_ = fs.Parse(rest)
		if id == "" && fs.NArg() > 0 {
			id = fs.Arg(0)
		}
		if id == "" {
			return fmt.Errorf("usage: mymcp client %s ID [--url URL]", args[0])
		}
		if args[0] == "detach" {
			return manager.Detach(id, *force)
		}
		if args[0] == "restore" {
			return manager.Restore(id)
		}
		if *direct {
			db, _, err := open()
			if err != nil {
				return err
			}
			defer db.Close()
			servers, err := db.ServersForProfile(context.Background(), *profile)
			if err != nil {
				return err
			}
			var directServers []configsync.DirectServer
			for _, server := range servers {
				directServers = append(directServers, configsync.DirectServer{
					Name: server.Name, Transport: server.Transport, Command: server.Command,
					Args: server.Args, URL: server.URL, Env: server.Env, Headers: server.Headers,
				})
			}
			if args[0] == "plan" {
				plan, err := manager.PlanDirect(id, directServers, *force)
				if err != nil {
					return err
				}
				raw, _ := json.MarshalIndent(plan, "", "  ")
				fmt.Println(string(raw))
				return nil
			}
			plan, err := manager.ApplyDirect(id, directServers, *force)
			if err != nil {
				return err
			}
			fmt.Printf("applied %d servers to %s (%s)\n", len(directServers), plan.Client, plan.File)
			return nil
		}
		endpoint := *url
		if endpoint == "" {
			db, _, err := open()
			if err != nil {
				return err
			}
			if _, err := db.Profile(context.Background(), *profile); err != nil {
				db.Close()
				return fmt.Errorf("profile %q not found", *profile)
			}
			db.Close()
			endpoint = "http://" + *addr + "/mcp/" + *profile
		}
		if args[0] == "plan" {
			plan, err := manager.Plan(id, endpoint, *entry)
			if err != nil {
				return err
			}
			raw, _ := json.MarshalIndent(plan, "", "  ")
			fmt.Println(string(raw))
			return nil
		}
		plan, err := manager.Apply(id, endpoint, *entry, *force)
		if err != nil {
			return err
		}
		fmt.Printf("applied %s to %s (%s)\n", plan.Entry, plan.Client, plan.File)
		return nil
	default:
		return fmt.Errorf("unknown client command %q", args[0])
	}
}

func doctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, secretStore, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	issues := 0
	profiles, err := db.ListProfiles(ctx)
	if err != nil {
		return err
	}
	if len(profiles) == 0 {
		fmt.Println("profiles: none (default is created on serve)")
	} else {
		for _, profile := range profiles {
			servers, err := db.ServersForProfile(ctx, profile.Name)
			if err != nil {
				return err
			}
			fmt.Printf("profile %s: %d servers\n", profile.Name, len(servers))
		}
	}
	servers, err := db.ListServers(ctx)
	if err != nil {
		return err
	}
	for _, server := range servers {
		switch server.Transport {
		case "stdio":
			if server.Executor == "docker" {
				if server.Image == "" {
					fmt.Printf("server %q has no Docker image\n", server.Name)
					issues++
				}
			} else if server.Command == "" {
				fmt.Printf("server %q has no command\n", server.Name)
				issues++
			}
		case "http", "sse":
			if !strings.HasPrefix(server.URL, "http://") && !strings.HasPrefix(server.URL, "https://") {
				fmt.Printf("server %q has an invalid URL %q\n", server.Name, server.URL)
				issues++
			}
		}
		if server.OAuth != nil && !secretStore.Has(oauth.Key(server.Name)) {
			fmt.Printf("server %q needs OAuth login\n", server.Name)
			issues++
		}
		for _, refs := range []map[string]string{server.Env, server.Headers} {
			for key, value := range refs {
				if !secrets.IsRef(value) {
					continue
				}
				name := secrets.Name(value)
				if !secretStore.Has(name) {
					fmt.Printf("missing secret %q for %s.%s\n", name, server.Name, key)
					issues++
				}
			}
		}
	}
	manager, err := configsync.New(home())
	if err != nil {
		return err
	}
	for _, status := range manager.Statuses() {
		if status.Managed && !status.Present {
			fmt.Printf("managed client %s is missing %s\n", status.Client, status.File)
			issues++
			continue
		}
		if status.Drifted {
			fmt.Printf("managed client %s changed since MyMCP wrote it: %s\n", status.Client, status.File)
			issues++
		}
	}
	if issues > 0 {
		return fmt.Errorf("%d issue(s) found", issues)
	}
	fmt.Println("ok: no missing secrets or managed client files")
	return nil
}

func key(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mymcp key create|list|revoke")
	}
	db, _, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("key create", flag.ExitOnError)
		profileName := fs.String("profile", "", "restrict the key to one profile")
		rate := fs.Int("rate", 0, "calls per minute, 0 for unlimited")
		expires := fs.String("expires", "", "lifetime, e.g. 24h or 30d")
		rest := args[1:]
		name := ""
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			name, rest = rest[0], rest[1:]
		}
		if err := fs.Parse(rest); err != nil {
			return err
		}
		if name == "" && fs.NArg() > 0 {
			name = fs.Arg(0)
		}
		if name == "" {
			return fmt.Errorf("usage: mymcp key create NAME")
		}
		expiry, err := parseExpiry(*expires)
		if err != nil {
			return err
		}
		if *profileName != "" {
			if _, err := db.Profile(ctx, *profileName); err != nil {
				return fmt.Errorf("profile %q not found", *profileName)
			}
		}
		token, created, err := db.CreateKey(ctx, name, *profileName, *rate, expiry)
		if err != nil {
			return err
		}
		fmt.Println(token)
		fmt.Fprintf(os.Stderr, "created key %s (profile %q, rate %d/min)\n", created.Name, created.Profile, created.RatePerMin)
		return nil
	case "list":
		keys, err := db.ListKeys(ctx)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tPROFILE\tRATE\tEXPIRES\tLAST USED")
		for _, k := range keys {
			expires := "never"
			if !k.ExpiresAt.IsZero() {
				expires = k.ExpiresAt.Format("2006-01-02 15:04")
			}
			last := "never"
			if !k.LastUsedAt.IsZero() {
				last = k.LastUsedAt.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(w, "%s\t%s\t%d/min\t%s\t%s\n", k.Name, k.Profile, k.RatePerMin, expires, last)
		}
		return w.Flush()
	case "revoke":
		if len(args) != 2 {
			return fmt.Errorf("usage: mymcp key revoke NAME")
		}
		return db.RevokeKey(ctx, args[1])
	default:
		return fmt.Errorf("unknown key command %q", args[0])
	}
}

func parseExpiry(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if days, err := strconv.Atoi(strings.TrimSuffix(value, "d")); err == nil && strings.HasSuffix(value, "d") {
		if days <= 0 {
			return time.Time{}, fmt.Errorf("expiry must be positive")
		}
		return time.Now().Add(time.Duration(days) * 24 * time.Hour), nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return time.Time{}, fmt.Errorf("invalid expiry %q", value)
	}
	return time.Now().Add(duration), nil
}

func oauthCommand(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: mymcp oauth login|status|logout SERVER")
	}
	db, secretStore, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	server, err := db.GetServer(ctx, args[1])
	if err != nil {
		return err
	}
	switch args[0] {
	case "discover":
		fs := flag.NewFlagSet("oauth discover", flag.ExitOnError)
		clientID := fs.String("client-id", "", "OAuth client ID (otherwise dynamic registration)")
		clientSecret := fs.String("client-secret", "", "OAuth client secret or secret://name")
		redirect := fs.String("redirect", "http://127.0.0.1:33418/callback", "redirect URI registered with the client")
		var scopes stringList
		fs.Var(&scopes, "scope", "OAuth scope, repeatable")
		_ = fs.Parse(args[2:])
		if server.URL == "" {
			return fmt.Errorf("%s has no URL to discover", server.Name)
		}
		resource, err := oauth.Discover(ctx, server.URL, http.DefaultClient)
		if err != nil {
			return err
		}
		issuer := resource.AuthorizationServers[0]
		auth, err := oauth.AuthServer(ctx, issuer, http.DefaultClient)
		if err != nil {
			return err
		}
		id, secret := *clientID, *clientSecret
		if id == "" {
			if auth.RegistrationEndpoint == "" {
				return fmt.Errorf("no registration endpoint; pass --client-id")
			}
			id, secret, err = oauth.Register(ctx, auth, *redirect, "MyMCP", http.DefaultClient)
			if err != nil {
				return err
			}
			fmt.Println("registered client", id)
		}
		if secret != "" && !secrets.IsRef(secret) {
			name := "oauth-client/" + server.Name
			if err := secretStore.Set(name, secret); err != nil {
				return err
			}
			secret = "secret://" + name
		}
		if len(scopes) == 0 {
			scopes = resource.Scopes
		}
		if len(scopes) == 0 {
			scopes = auth.Scopes
		}
		cfg := &store.OAuthConfig{
			AuthorizationURL: auth.AuthorizationEndpoint, TokenURL: auth.TokenEndpoint,
			ClientID: id, ClientSecret: secret, Scopes: scopes, RedirectURI: *redirect, UsePKCE: true,
		}
		if err := db.SetServerOAuth(ctx, server.Name, cfg); err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", server.Name, auth.AuthorizationEndpoint)
		fmt.Println("now run: mymcp oauth login", server.Name)
		return nil
	case "login":
		if server.OAuth == nil {
			return fmt.Errorf("%s has no OAuth configuration; add it with --oauth-* flags", server.Name)
		}
		token, err := oauth.Login(ctx, server.OAuth, server.Name, secretStore, openBrowser, os.Stdout)
		if err != nil {
			return err
		}
		fmt.Printf("signed in %s (token %s…)\n", server.Name, mask(token.AccessToken))
		return nil
	case "status":
		token, err := oauth.Load(secretStore, server.Name)
		if err != nil {
			return fmt.Errorf("%s: not signed in", server.Name)
		}
		state := "valid"
		if !token.Expiry.IsZero() && time.Now().After(token.Expiry) {
			state = "expired (refresh on next use)"
		}
		fmt.Printf("%s: %s, token %s…, expiry %s\n", server.Name, state, mask(token.AccessToken), token.Expiry.Format(time.RFC3339))
		return nil
	case "logout":
		return oauth.Delete(secretStore, server.Name)
	default:
		return fmt.Errorf("unknown oauth command %q", args[0])
	}
}

func openBrowser(target string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", target).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	default:
		return exec.Command("xdg-open", target).Start()
	}
}

func mask(value string) string {
	if len(value) <= 8 {
		return "****"
	}
	return value[:4] + "…" + value[len(value)-4:]
}

func pluginCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mymcp plugin list|install DIR|remove ID")
	}
	dir := filepath.Join(home(), "plugins")
	switch args[0] {
	case "list":
		registry, err := agent.New(home())
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tLABEL\tVERSION\tBUILTIN\tCONFIG")
		for _, adapter := range registry.Agents() {
			fmt.Fprintf(w, "%s\t%s\t%s\t%t\t%s\n", adapter.ID(), adapter.Label(), adapter.Plugin.Manifest.Version, adapter.Builtin, adapter.Path())
		}
		if err := w.Flush(); err != nil {
			return err
		}
		for _, err := range registry.Errors() {
			fmt.Fprintln(os.Stderr, "plugin:", err)
		}
		return nil
	case "install":
		if len(args) != 2 {
			return fmt.Errorf("usage: mymcp plugin install DIR")
		}
		installed, err := plugin.Install(args[1], dir)
		if err != nil {
			return err
		}
		fmt.Println("installed", installed.Manifest.ID, "into", filepath.Join(dir, installed.Manifest.ID))
		return nil
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: mymcp plugin remove ID")
		}
		return plugin.Remove(dir, args[1])
	default:
		return fmt.Errorf("unknown plugin command %q", args[0])
	}
}

func importServers(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	profileName := fs.String("profile", "default", "profile to add imported servers to")
	clientID := fs.String("client", "", "only this client")
	only := fs.String("only", "", "comma-separated server names to import")
	dry := fs.Bool("dry-run", false, "show what would be imported")
	if err := fs.Parse(args); err != nil {
		return err
	}
	db, _, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	sync, err := configsync.New(home())
	if err != nil {
		return err
	}
	var selected map[string]bool
	if value := strings.TrimSpace(*only); value != "" {
		selected = map[string]bool{}
		for _, name := range strings.Split(value, ",") {
			if name = strings.TrimSpace(name); name != "" {
				selected[name] = true
			}
		}
	}
	stats, err := importer.ImportSelected(context.Background(), db, sync, *profileName, *clientID, selected, *dry)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, result := range stats.Results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", result.Name, result.Client, result.Status, result.Error)
	}
	w.Flush()
	fmt.Printf("imported %d, existing %d, duplicate %d, invalid %d\n", stats.Imported, stats.Existing, stats.Duplicate, stats.Invalid)
	return nil
}
