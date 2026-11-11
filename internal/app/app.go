package app

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/tk103331/mymcp/internal/configsync"
	"github.com/tk103331/mymcp/internal/data"
	"github.com/tk103331/mymcp/internal/gateway"
	"github.com/tk103331/mymcp/internal/importer"
	"github.com/tk103331/mymcp/internal/manager"
	"github.com/tk103331/mymcp/internal/plugin"
	"github.com/tk103331/mymcp/internal/secrets"
	"github.com/tk103331/mymcp/internal/store"
	"github.com/tk103331/mymcp/locales"
)

const (
	sidebarWidth = 236
	titleBarMin  = 48
)

// app is the window's state and its view.
type app struct {
	win      *mygo.Window
	tray     *mygo.Tray
	trayMenu *mygo.Menu
	quitting bool
	router   *ui.Router

	settings        data.Settings
	workspaces      []*data.Workspace
	workspaceCounts map[string]int
	catalog         *catalogData

	current   *data.Workspace
	instances []*data.ServerInstance
	lastPath  string

	usage        *store.DB
	secretsStore *secrets.Store
	usageSummary *store.UsageSummary
	usageCalls   []*store.Call
	usageSince   time.Duration
	usageError   string

	gwManager        *gateway.Manager
	gwServer         *http.Server
	gwRunning        bool
	gwStarting       bool
	gwError          string
	gwAddr           string
	gwServers        []*store.Server
	gwProfiles       []*store.Profile
	gwCounts         map[string]int
	gwKeys           []*store.GatewayKey
	gwStatus         map[string]gateway.Status
	gwServerProfiles map[string][]string
	gwServerErrors   map[string]string
	gwServerQuery    string
	gwDirty          bool

	gwAddOpen       bool
	gwEditing       string
	gwTab           int
	gwName          string
	gwTransport     string
	gwCommand       string
	gwArgs          string
	gwURL           string
	gwProfile       string
	gwEnvText       string
	gwHeadersText   string
	gwOAuth         bool
	gwAuthURL       string
	gwTokenURL      string
	gwClientID      string
	gwClientSecret  string
	gwOAuthRedirect string
	gwScopesText    string
	gwPKCE          bool
	gwOAuthBusy     bool
	gwOAuthNotice   string
	gwOAuthError    string
	gwExecutor      string
	gwImage         string
	gwNetwork       string
	gwVolumesText   string
	gwDelete        *store.Server
	gwKeyOpen       bool
	gwKeyName       string
	gwKeyProfile    string
	gwKeyExpiry     string
	gwKeyRateText   string
	gwKeyToken      string
	gwKeyError      string
	gwKeyRevoke     *store.GatewayKey
	gwImportOpen    bool
	gwImportStats   *importer.Stats
	gwImportError   string

	gwProfileOpen    bool
	gwProfileEditing string
	gwProfileName    string
	gwProfileDesc    string
	gwProfileServers map[string]bool
	gwProfileError   string

	syncManager       *configsync.Manager
	clientStatuses    []configsync.Status
	clientProfile     string
	clientPlan        *configsync.Plan
	clientPlanOpen    bool
	clientPlanClient  string
	clientDetachForce string

	debugProfile  string
	debugTools    []gateway.ToolInfo
	debugQuery    string
	debugSelected string
	debugArgs     string
	debugResult   string
	debugError    string
	debugLatency  int64
	debugBusy     bool
	debugList     ui.ListState

	pluginList   []*plugin.Plugin
	pluginErrors []error
	pluginRemove *plugin.Plugin
	pluginNotice string
	pluginError  string

	secretNames     []string
	secretError     string
	secretNotice    string
	secretFormOpen  bool
	secretEditing   string
	secretName      string
	secretValue     string
	secretFormError string
	secretDelete    string

	// Workspace dialogs.
	createOpen  bool
	newName     string
	newDesc     string
	newAutoRun  bool
	createError string
	deleteOpen  bool

	// Add service dialog.
	addOpen      bool
	addStep      int
	addQuery     string
	addCategory  string
	addSelected  string
	addType      string
	addParams    map[string]string
	addError     string
	addList      ui.ListState
	discoverList ui.ListState

	// Managed client dialog.
	configOpen bool
	configTab  int

	// Connection configuration dialog.
	connectOpen     bool
	connectInstance *data.ServerInstance
	connectTab      int

	// Discover.
	query    string
	category string
	detail   string

	// Install dialog.
	installOpen    bool
	installSlug    string
	installProfile string
	installType    string
	installParms   map[string]string
	installError   string

	// Settings choices, as the labels the select shows.
	languageChoice string
	themeChoice    string
	settingsError  string
}

// New creates the app's state.
func New() *app {
	if version := mygo.App.Version(); version != "" {
		gateway.Version = version
	}
	settings, err := data.LoadSettings()
	if err != nil {
		log.Println("load settings:", err)
		settings = &data.Settings{Language: "zh", Theme: "system", BaseURL: "http://localhost:8421", GatewayAddr: "127.0.0.1:8422"}
	}
	loadClientIcons()
	a := &app{
		router:        ui.NewRouter("/overview"),
		settings:      *settings,
		catalog:       loadCatalog(),
		usageSince:    24 * time.Hour,
		gwAddr:        settings.GatewayAddr,
		clientProfile: "default",
	}
	if addr, err := normalizeGatewayAddr(a.gwAddr); err == nil {
		a.gwAddr = addr
	} else {
		a.gwAddr = "127.0.0.1:8422"
	}
	a.settings.GatewayAddr = a.gwAddr
	if dir := mymcpDir(); dir != "" {
		if manager, err := configsync.New(dir); err == nil {
			a.syncManager = manager
		} else {
			log.Println("client sync:", err)
		}
	}
	a.loadWorkspaces()
	if dir := mymcpDir(); dir != "" {
		if secretStore, err := secrets.Open(dir); err == nil {
			a.secretsStore = secretStore
		} else {
			log.Println("secret store:", err)
		}
		if db, err := store.Open(filepath.Join(dir, "mymcp.db")); err == nil {
			a.usage = db
		} else {
			log.Println("usage database:", err)
		}
	}
	a.configureBridge()
	return a
}

// configureBridge copies the bundled mcp-pipe next to the app's data and
// points bridged client configs at that stable path. The helper survives
// app updates and works without mcp-pipe being on PATH.
func (a *app) configureBridge() {
	dir := mymcpDir()
	if dir == "" {
		return
	}
	name := "mcp-pipe"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	target := filepath.Join(dir, "bin", name)
	if resources, err := mygo.App.Path(mygo.PathResources); err == nil {
		source := filepath.Join(resources, "bin", name)
		if info, err := os.Stat(source); err == nil && info.Mode().IsRegular() {
			if !sameHelper(source, target) {
				if err := copyHelper(source, target); err != nil {
					log.Println("bridge helper:", err)
				}
			}
		}
	}
	if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
		configsync.SetBridgeCommand(target)
	}
}

func sameHelper(source, target string) bool {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return false
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		return false
	}
	if sourceInfo.Size() != targetInfo.Size() {
		return false
	}
	if sourceInfo.ModTime().After(targetInfo.ModTime()) {
		return false
	}
	sourceRaw, err := os.ReadFile(source)
	if err != nil {
		return false
	}
	targetRaw, err := os.ReadFile(target)
	if err != nil {
		return false
	}
	return bytes.Equal(sourceRaw, targetRaw)
}

func copyHelper(source, target string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	temp := target + ".tmp"
	if err := os.WriteFile(temp, raw, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(temp, 0o755); err != nil {
		os.Remove(temp)
		return err
	}
	return os.Rename(temp, target)
}

// Run starts the application.
func (a *app) Run() error {
	a.applyTheme()
	a.applyLocale()

	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting = true })
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			a.showWindow()
		}
	})
	mygo.App.OnSecondInstance(func([]string, string) { a.showWindow() })
	mygo.App.OnLocaleChanged(func(locale string) {
		a.languageChoice, a.themeChoice = "", ""
		if a.win != nil {
			a.win.SetTitle(locales.In(locale).AppTitle())
		}
		a.refreshTrayMenu()
	})
	mygo.App.WhenReady(func() {
		// The system is the source of truth for the login item: the user may
		// have turned it off in the system's settings.
		a.settings.LaunchAtLogin = mygo.App.OpenAtLogin()
		_ = data.SaveSettings(&a.settings)

		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:           locales.AppTitle(),
			Width:           1180,
			Height:          780,
			MinWidth:        940,
			MinHeight:       620,
			TitleBarStyle:   mygo.TitleBarHidden,
			Vibrancy:        mygo.VibrancySidebar,
			BackgroundColor: "light-dark(#ffffff, #1c1c1e)",
			StateKey:        "main",
			Hidden:          mygo.App.WasOpenedAtLogin(),
			Content:         ui.View(a.view),
		})
		// Closing the window keeps the app in the menu bar; quit from there
		// or with Cmd+Q, which OnBeforeQuit lets through.
		a.win.OnClose(func(e *mygo.CloseEvent) {
			if !a.quitting {
				e.PreventDefault()
				a.win.Hide()
			}
		})
		a.setUpTray()
		a.startGateway(locales.In(mygo.App.Locale()))
		go func() {
			manager.InitHttpServer()
			// Workspaces marked to start with the app start once here, not on
			// every HTTP server restart.
			manager.StartAutoRunWorkspaces()
			if err := manager.StartHttpServer(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Println("http server:", err)
			}
		}()
	})
	return mygo.App.Run()
}

// shutdown releases the gateway, workspace processes and the database.
func (a *app) shutdown() {
	if a.gwServer != nil {
		_ = a.gwServer.Close()
	}
	if a.gwManager != nil {
		a.gwManager.Close()
	}
	manager.StopHttpServer()
	manager.StopAll()
	if a.usage != nil {
		_ = a.usage.Close()
		a.usage = nil
	}
}

// showWindow brings the main window back from the menu bar.
func (a *app) showWindow() {
	if a.win == nil {
		return
	}
	a.win.Show()
	a.win.Focus()
}

// setUpTray adds the menu bar icon. A platform without a tray keeps the app
// usable through its window and Dock icon.
func (a *app) setUpTray() {
	menu := a.buildTrayMenu()
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "main",
		Icon:           trayIcon(),
		IconIsTemplate: true,
		ToolTip:        locales.In(mygo.App.Locale()).TrayTooltip(),
		Menu:           menu,
	})
	if err != nil {
		log.Println("tray:", err)
		return
	}
	a.tray, a.trayMenu = tray, menu
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		a.shutdown()
		a.tray.Destroy()
	})
}

func (a *app) buildTrayMenu() *mygo.Menu {
	msgs := locales.In(mygo.App.Locale())
	return mygo.NewMenu([]*mygo.MenuItem{
		{ID: "open", Label: msgs.TrayOpen(), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.showWindow()
		}},
		{ID: "settings", Label: msgs.TraySettings(), Click: func(*mygo.MenuItem, *mygo.Window) {
			a.router.Push("/settings")
			a.showWindow()
		}},
		mygo.Separator(),
		{ID: "login", Label: msgs.LaunchAtLogin(), Type: mygo.MenuItemCheckbox,
			Checked: a.settings.LaunchAtLogin,
			Click: func(item *mygo.MenuItem, _ *mygo.Window) {
				if err := a.setLaunchAtLogin(item.Checked); err != nil {
					log.Println("launch at login:", err)
					item.SetChecked(!item.Checked)
				}
			}},
		mygo.Separator(),
		{ID: "quit", Label: msgs.TrayQuit(), Click: func(*mygo.MenuItem, *mygo.Window) {
			mygo.App.Quit()
		}},
	})
}

func (a *app) refreshTrayMenu() {
	if a.tray == nil {
		return
	}
	a.trayMenu = a.buildTrayMenu()
	a.tray.SetMenu(a.trayMenu)
	a.tray.SetToolTip(locales.In(mygo.App.Locale()).TrayTooltip())
}

func (a *app) setLaunchAtLogin(enabled bool) error {
	if err := mygo.App.SetOpenAtLogin(enabled); err != nil {
		return err
	}
	a.settings.LaunchAtLogin = enabled
	if err := data.SaveSettings(&a.settings); err != nil {
		log.Println("save settings:", err)
	}
	if a.trayMenu != nil {
		if item := a.trayMenu.ItemByID("login"); item != nil {
			item.SetChecked(enabled)
		}
	}
	return nil
}

// normalizeGatewayAddr accepts a loopback host and port for the gateway.
func normalizeGatewayAddr(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	host, port, err := net.SplitHostPort(raw)
	if err != nil || host == "" || port == "" {
		return "", fmt.Errorf("invalid address")
	}
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return "", fmt.Errorf("not a loopback address")
		}
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", fmt.Errorf("invalid port")
	}
	return net.JoinHostPort(host, port), nil
}

// queueTheme applies the theme after the frame that changed it, as
// Theme.SetSource may redraw the app and must not run inside a build.
func (a *app) queueTheme() {
	if a.win == nil {
		return
	}
	a.win.Update(a.applyTheme)
}

func (a *app) applyTheme() {
	switch a.settings.Theme {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

func (a *app) applyLocale() {
	switch a.settings.Language {
	case "zh":
		mygo.App.SetLocale("zh-Hans")
	case "en":
		mygo.App.SetLocale("en")
	default:
		mygo.App.SetLocale("")
	}
}

// view builds the window.
func (a *app) view(c *ui.Context) {
	t := c.Theme()
	msgs := locales.In(c.Locale())
	vibrant := c.Vibrancy()
	if vibrant {
		c.Root().Background(ui.Transparent)
	}

	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	row.Children(func() {
		a.sidebar(c, t, msgs, vibrant)
		ui.Box(c).Width(1).Background(t.Border)
		a.content(c, t, msgs)
	})
	a.dialogs(c, t, msgs)
}

func (a *app) sidebar(c *ui.Context, t *ui.Theme, msgs locales.Messages, vibrant bool) {
	bar := c.TitleBar()
	side := ui.Column(c).Width(sidebarWidth).Shrink(0)
	if !vibrant {
		side.Background(t.Surface)
	}
	side.Children(func() {
		ui.Box(c).Height(maxf(bar.Height, titleBarMin)).Shrink(0).DragWindow()

		ui.Row(c).Padding(18, 16, 8, 16).Gap(11).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(34, 34).Radius(10).Background(t.Accent.Alpha(0.13)).
				Center().Children(func() {
				ui.Icon(c, iconLogo).Size(21, 21).TextColor(t.Accent)
			})
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, msgs.AppTitle()).FontSize(14.5).Bold()
				ui.Text(c, msgs.SidebarSubtitle()).FontSize(10).TextColor(t.TextMuted)
			})
		})

		section := a.section()
		sb := ui.Sidebar(c, &section, func() {
			ui.SidebarItem(c, "overview", iconHome, msgs.NavOverview())
			item := ui.SidebarItem(c, "servers", iconServer, msgs.NavServers())
			if len(a.gwServers) > 0 {
				item.Children(func() {
					ui.Textf(c, "%d", len(a.gwServers)).FontSize(10.5).TextColor(t.TextMuted)
				})
			}
			item = ui.SidebarItem(c, "catalog", iconSearch, msgs.NavCatalog())
			item.Children(func() {
				ui.Textf(c, "%d", len(a.catalog.Servers())).FontSize(10.5).TextColor(t.TextMuted)
			})
			item = ui.SidebarItem(c, "profiles", iconLayers, msgs.NavProfiles())
			if len(a.gwProfiles) > 0 {
				item.Children(func() {
					ui.Textf(c, "%d", len(a.gwProfiles)).FontSize(10.5).TextColor(t.TextMuted)
				})
			}
			ui.SidebarItem(c, "clients", iconLink, msgs.NavClients())
			ui.SidebarItem(c, "debug", iconDebug, msgs.NavDebug())
			ui.SidebarItem(c, "secrets", iconKey, msgs.NavSecrets())
			ui.SidebarItem(c, "plugins", iconPuzzle, msgs.NavPlugins())
			ui.SidebarItem(c, "usage", iconChart, msgs.NavUsage())
			ui.SidebarItem(c, "settings", iconSettings, msgs.NavSettings())
		})
		if vibrant {
			sb.Background(ui.Transparent)
		}
		sb.Grow(1).Label(msgs.AppTitle())
		if sb.Changed() {
			a.router.Push("/" + section)
		}

		ui.Divider(c).MarginX(12)
		ui.Column(c).Padding(10, 12, 12, 12).Gap(7).Shrink(0).Children(func() {
			dot := t.TextMuted
			switch {
			case a.gwRunning:
				dot = t.Success
			case a.gwError != "":
				dot = t.Danger
			}
			endpoint := ui.ButtonBase(c).Padding(6, 8).Radius(8).Background(t.Background.Alpha(0.45)).
				Tooltip(msgs.CopyEndpoint())
			endpoint.Children(func() {
				ui.Box(c).Size(6, 6).Radius(999).Background(dot)
				ui.Text(c, a.gwAddr).FontSize(10.5).TextColor(t.TextMuted).SingleLine()
				ui.Spacer(c)
				ui.Icon(c, iconCopy).Size(11, 11).TextColor(t.TextMuted)
			})
			if endpoint.Clicked() {
				mygo.Clipboard.WriteText("http://" + a.gwAddr)
				c.Toast(msgs.EndpointCopied())
			}
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, appVersion()).FontSize(10.5).TextColor(t.TextMuted)
				ui.Spacer(c)
				ui.Text(c, "MyGo").FontSize(10.5).TextColor(t.TextMuted).Opacity(0.6)
			})
		})
	})
}

func (a *app) content(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	bar := c.TitleBar()
	main := ui.Column(c).Grow(1).MinWidth(0).Background(t.Background)
	main.Children(func() {
		ui.Row(c).Height(maxf(bar.Height, titleBarMin)).Shrink(0).AlignItems(ui.Center).
			Padding(0, bar.Right+20, 0, 24).Gap(10).DragWindow().Children(func() {
			ui.Text(c, a.windowTitle(msgs)).FontSize(13).Bold().TextColor(t.TextMuted)
			ui.Spacer(c)
			a.runningPill(c, t, msgs)
			a.gatewayPill(c, t, msgs)
		})
		ui.Divider(c)

		path := a.router.Path()
		if path != a.lastPath {
			a.lastPath = path
			if !strings.HasPrefix(path, "/workspaces/") {
				a.current, a.instances = nil, nil
			}
			if path == "/usage" {
				a.loadUsage()
			}
			if path == "/overview" || path == "/gateway" {
				a.loadOverview()
			}
			if path == "/servers" || path == "/profiles" {
				a.loadGateway()
			}
			if path == "/debug" {
				a.loadDebug()
			}
			if path == "/clients" {
				a.loadClients()
			}
			if path == "/secrets" {
				a.loadSecrets()
			}
			if path == "/plugins" {
				a.loadPlugins()
			}
		}
		a.router.View(c, func(r *ui.Route) {
			switch {
			case r.Match("/overview"), r.Match("/gateway"):
				r.Title(msgs.OverviewTitle())
				a.overviewPage(c, t, msgs)
			case r.Match("/servers"):
				r.Title(msgs.ServersTitle())
				a.serversPage(c, t, msgs)
			case r.Match("/profiles"):
				r.Title(msgs.ProfilesTitle())
				a.profilesPage(c, t, msgs)
			case r.Match("/catalog"), r.Match("/discover"):
				r.Title(msgs.CatalogTitle())
				a.discoverPage(c, t, msgs)
			case r.Match("/catalog/{name}"), r.Match("/discover/{name}"):
				r.Title(msgs.CatalogTitle())
				a.detailPage(c, t, msgs, r.Param("name"))
			case r.Match("/debug"):
				r.Title(msgs.DebugTitle())
				a.debugPage(c, t, msgs)
			case r.Match("/clients"):
				r.Title(msgs.ClientsTitle())
				a.clientsPage(c, t, msgs)
			case r.Match("/secrets"):
				r.Title(msgs.SecretsTitle())
				a.secretsPage(c, t, msgs)
			case r.Match("/plugins"):
				r.Title(msgs.PluginsTitle())
				a.pluginsPage(c, t, msgs)
			case r.Match("/usage"):
				r.Title(msgs.UsageTitle())
				a.usagePage(c, t, msgs)
			case r.Match("/settings"):
				r.Title(msgs.SettingsTitle())
				a.settingsPage(c, t, msgs)
			// Kept for compatibility with older deep links.
			case r.Match("/workspaces"):
				r.Title(msgs.WorkspacesTitle())
				a.workspacesPage(c, t, msgs)
			case r.Match("/workspaces/{id}"):
				r.Title(msgs.WorkspacesTitle())
				a.workspaceDetail(c, t, msgs, r.Param("id"))
			default:
				ui.Text(c, msgs.NoResults()).Padding(24)
			}
		})
	})
}

// gatewayPill keeps the gateway state visible outside its page, especially
// when a port conflict stopped it from starting.
func (a *app) gatewayPill(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	text, color := "", t.TextMuted
	switch {
	case a.gwStarting:
		text, color = msgs.NavGateway()+" · "+msgs.GatewayStarting(), t.Warning
	case a.gwRunning:
		text, color = msgs.NavGateway()+" · "+msgs.GatewayRunning(), t.Success
	case a.gwError != "":
		text, color = msgs.NavGateway()+" · "+msgs.StatusError(), t.Danger
	default:
		return
	}
	pill := ui.Row(c).Padding(3, 9).Radius(999).Background(color.Alpha(0.13)).
		AlignItems(ui.Center).Gap(6)
	if a.gwError != "" {
		pill.Tooltip(a.gwError)
	}
	pill.Children(func() {
		ui.Box(c).Size(6, 6).Radius(999).Background(color)
		ui.Text(c, text).FontSize(10.5).TextColor(color)
	})
}

func (a *app) runningPill(c *ui.Context, t *ui.Theme, msgs locales.Messages) {
	if !strings.HasPrefix(a.router.Path(), "/workspaces/") {
		return
	}
	running := 0
	for _, instance := range a.instances {
		if instance.Status == "started" {
			running++
		}
	}
	if running == 0 {
		return
	}
	ui.Row(c).Padding(3, 9).Radius(999).Background(t.Success.Alpha(0.13)).
		AlignItems(ui.Center).Gap(6).Children(func() {
		ui.Box(c).Size(6, 6).Radius(999).Background(t.Success)
		ui.Text(c, msgs.RunningCount(running)).FontSize(10.5).TextColor(t.Success)
	})
}

func (a *app) section() string {
	path := a.router.Path()
	switch {
	case strings.HasPrefix(path, "/servers"):
		return "servers"
	case strings.HasPrefix(path, "/profiles"):
		return "profiles"
	case strings.HasPrefix(path, "/catalog"), strings.HasPrefix(path, "/discover"):
		return "catalog"
	case strings.HasPrefix(path, "/settings"):
		return "settings"
	case strings.HasPrefix(path, "/usage"):
		return "usage"
	case strings.HasPrefix(path, "/debug"):
		return "debug"
	case strings.HasPrefix(path, "/clients"):
		return "clients"
	case strings.HasPrefix(path, "/secrets"):
		return "secrets"
	case strings.HasPrefix(path, "/plugins"):
		return "plugins"
	default:
		return "overview"
	}
}

func (a *app) windowTitle(msgs locales.Messages) string {
	path := a.router.Path()
	switch {
	case path == "/overview", path == "/gateway":
		return msgs.OverviewTitle()
	case path == "/servers":
		return msgs.ServersTitle()
	case path == "/profiles":
		return msgs.ProfilesTitle()
	case path == "/catalog", path == "/discover":
		return msgs.CatalogTitle()
	case strings.HasPrefix(path, "/catalog/"), strings.HasPrefix(path, "/discover/"):
		if server := a.catalog.Server(a.detail); server != nil {
			return server.DisplayName
		}
		return msgs.CatalogTitle()
	case path == "/settings":
		return msgs.SettingsTitle()
	case path == "/usage":
		return msgs.UsageTitle()
	case path == "/debug":
		return msgs.DebugTitle()
	case path == "/clients":
		return msgs.ClientsTitle()
	case path == "/secrets":
		return msgs.SecretsTitle()
	case path == "/plugins":
		return msgs.PluginsTitle()
	case strings.HasPrefix(path, "/workspaces/"):
		if a.current != nil {
			return a.current.Name
		}
		return msgs.WorkspacesTitle()
	default:
		return msgs.OverviewTitle()
	}
}

func (a *app) loadWorkspaces() {
	workspaces, err := data.LoadWorkspaces()
	if err != nil {
		log.Println("load workspaces:", err)
		return
	}
	a.workspaces = workspaces
	counts := map[string]int{}
	if configs, err := data.LoadServerConfigs(); err == nil {
		for _, config := range configs {
			counts[config.Workspace]++
		}
	}
	a.workspaceCounts = counts
}

// loadWorkspace loads a workspace and its instances once the page shows another.
func (a *app) loadWorkspace(id string) {
	if a.current != nil && a.current.ID == id {
		return
	}
	workspace, err := data.GetWorkspace(id)
	if err != nil {
		log.Println("load workspace:", err)
		a.current = nil
		a.instances = nil
		return
	}
	a.current = workspace
	a.reloadInstances()
}

func (a *app) reloadInstances() {
	if a.current == nil {
		a.instances = nil
		return
	}
	configs, err := data.GetWorkspaceServerConfigs(a.current.ID)
	if err != nil {
		log.Println("load workspace servers:", err)
		a.instances = nil
		return
	}
	instances := make([]*data.ServerInstance, 0, len(configs))
	for _, cfg := range configs {
		instance, err := manager.GetServerInstance(cfg.ID)
		if err != nil || instance.Config == nil {
			instance = &data.ServerInstance{
				ID:     cfg.ID,
				Config: cfg,
				Status: "stopped",
			}
		}
		if instance.Status == "" {
			instance.Status = "stopped"
		}
		instances = append(instances, instance)
	}
	a.instances = instances
}

func (a *app) syncManagedClients() error {
	if a.current == nil {
		return nil
	}
	var first error
	for name := range a.current.ManagedClients {
		client := clientByName(name)
		if client == nil || client.ConfigPath() == "" {
			continue
		}
		if err := saveManagedClientConfig(client, a.instances); err != nil {
			log.Println("sync", name, ":", err)
			if first == nil {
				first = err
			}
		}
	}
	return first
}

func (a *app) addServerConfig(workspaceID, name, kind string, params map[string]string) error {
	server := a.catalog.Server(name)
	if server == nil {
		return errUnknownServer
	}
	cfg := modelServerConfig(workspaceID, server, kind, params)
	if strings.TrimSpace(cfg.Cmd) == "" {
		return fmt.Errorf("%s has no command for %s", server.DisplayName, kind)
	}
	if err := data.SaveServerConfig(&cfg); err != nil {
		return err
	}
	a.reloadInstances()
	return a.syncManagedClients()
}

func (a *app) startInstance(id string) error {
	err := manager.StartServerInstance(id)
	a.reloadInstances()
	return err
}

func (a *app) stopInstance(id string) error {
	err := manager.StopServerInstance(id)
	a.reloadInstances()
	return err
}

// appVersion is the version MyGo packs, or "dev" outside a build.
func appVersion() string {
	if version := mygo.App.Version(); version != "" {
		return "v" + version
	}
	return "dev"
}

func mymcpDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mymcp")
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
