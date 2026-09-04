package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pnsrc/ttgo/internal/client"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/gen2brain/beeep"
	"golang.design/x/hotkey"
)

// App — Wails-bound объект. Все экспортируемые методы доступны из frontend
// через wails-сгенерированные TS bindings (frontend/wailsjs/...).
type App struct {
	ctx      context.Context
	client   *client.Client
	profiles *client.ProfileStore
	settings *client.SettingsStore

	activeProfileID string // профиль с которым последний раз делали Connect
	pendingDeepLink string // enroll URL из deep link (firetunnel://enroll?url=...)
}

func NewApp() *App {
	store, err := client.NewProfileStore()
	if err != nil {
		slog.Warn("profile store init", "err", err)
	}
	settings, err := client.NewSettingsStore()
	if err != nil {
		slog.Warn("settings store init", "err", err)
	}
	return &App{
		client:   client.New(),
		profiles: store,
		settings: settings,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	client.RecoverDNS()

	go func() {
		results := client.CheckAllEnrollments(a.profiles)
		for _, r := range results {
			if r.Revoked {
				slog.Warn("enrolled device revoked", "message", r.Message)
			}
		}
	}()
}

func (a *App) domReady(_ context.Context) {
	go a.pushLoop()

	if a.settings != nil {
		go func() {
			if s, err := a.settings.Load(); err == nil && s.AutoConnect && s.LastProfileID != "" {
				// delay slightly to allow frontend to load
				time.Sleep(200 * time.Millisecond)
				slog.Info("auto connecting to last profile", "profile_id", s.LastProfileID)
				if err := a.ConnectProfile(s.LastProfileID); err != nil {
					slog.Warn("auto connect failed", "profile_id", s.LastProfileID, "err", err)
				}
			}
		}()
	}

	go a.registerHotkeys()
}

func (a *App) registerHotkeys() {
	mods, label := hotkeyMods()
	hk := hotkey.New(mods, hotkey.KeyV)
	err := hk.Register()
	if err != nil {
		slog.Warn("failed to register hotkey", "err", err)
		return
	}
	slog.Info("registered global hotkey", "combo", label)

	for {
		select {
		case <-a.ctx.Done():
			hk.Unregister()
			return
		case <-hk.Keydown():
			snap := a.client.Status()
			if snap.State == "connected" || snap.State == "connecting" {
				a.Disconnect()
			} else if a.activeProfileID != "" {
				a.ConnectProfile(a.activeProfileID)
			} else {
				if s, err := a.settings.Load(); err == nil && s.LastProfileID != "" {
					a.ConnectProfile(s.LastProfileID)
				}
			}
		}
	}
}

func (a *App) shutdown(_ context.Context) {
	_ = a.client.Disconnect()
}

func (a *App) pushLoop() {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
			wruntime.EventsEmit(a.ctx, "status", a.statusPayload())
		}
	}
}

// statusPayload — snapshot + active profile id (нужно UI для подсветки).
func (a *App) statusPayload() map[string]any {
	return map[string]any{
		"snapshot":          a.client.Status(),
		"active_profile_id": a.activeProfileID,
	}
}

// ── public bindings (callable from TS) ───────────────────────────────────────

// Status — для первой загрузки UI (потом приходит через event "status").
func (a *App) Status() map[string]any { return a.statusPayload() }

// Connect напрямую с raw Config (для form-based connect).
func (a *App) Connect(cfg client.Config) error {
	slog.Info("ui connect request", "endpoint", cfg.Endpoint, "user", cfg.Username)
	a.activeProfileID = ""

	if a.settings != nil {
		if s, err := a.settings.Load(); err == nil {
			if s.BypassDomains && len(s.GlobalExclusions) > 0 {
				cfg.Exclusions = append(cfg.Exclusions, s.GlobalExclusions...)
			}
			cfg.EnableAdBlock = s.EnableAdBlock
			if s.UpstreamDNS != "" {
				cfg.UpstreamDNS = s.UpstreamDNS
			} else {
				cfg.UpstreamDNS = "1.1.1.1" // default
			}
			cfg.RoutingMode = s.RoutingMode
		}
	}

	if err := a.client.Connect(cfg); err != nil {
		slog.Error("ui connect failed", "endpoint", cfg.Endpoint, "err", err)
		_ = beeep.Notify("FireTunnel", "Connection failed", "")
		return err
	}
	slog.Info("ui connect success", "endpoint", cfg.Endpoint)
	_ = beeep.Notify("FireTunnel", "Connected", "")
	return nil
}

// ConnectProfile открывает соединение по сохранённому профилю.
func (a *App) ConnectProfile(id string) error {
	if a.profiles == nil {
		return fmt.Errorf("profile store unavailable")
	}
	p, err := a.profiles.Get(id)
	if err != nil {
		return err
	}
	slog.Info("ui connect profile request", "profile_id", id, "endpoint", p.Endpoint, "user", p.Username)
	a.activeProfileID = id
	cfg := p.ToConfig()

	if a.settings != nil {
		if s, err := a.settings.Load(); err == nil {
			if s.BypassDomains && len(s.GlobalExclusions) > 0 {
				cfg.Exclusions = append(cfg.Exclusions, s.GlobalExclusions...)
			}
			cfg.EnableAdBlock = s.EnableAdBlock
			if s.UpstreamDNS != "" {
				cfg.UpstreamDNS = s.UpstreamDNS
			} else {
				cfg.UpstreamDNS = "1.1.1.1" // default
			}
			cfg.RoutingMode = s.RoutingMode
			s.LastProfileID = id
			_ = a.settings.Save(s)
		}
	}

	if err := a.client.Connect(cfg); err != nil {
		a.activeProfileID = ""
		slog.Error("ui connect profile failed", "profile_id", id, "err", err)
		_ = beeep.Notify("FireTunnel", "Connection failed", "")
		return err
	}
	slog.Info("ui connect profile success", "profile_id", id, "endpoint", p.Endpoint)
	_ = beeep.Notify("FireTunnel", "Connected: "+p.Name, "")
	return nil
}

func (a *App) Disconnect() error {
	slog.Info("ui disconnect request", "active_profile_id", a.activeProfileID)
	a.activeProfileID = ""
	if err := a.client.Disconnect(); err != nil {
		slog.Error("ui disconnect failed", "err", err)
		return err
	}
	slog.Info("ui disconnect success")
	_ = beeep.Notify("FireTunnel", "Disconnected", "")
	return nil
}

// Profiles — список всех сохранённых профилей.
func (a *App) Profiles() ([]client.Profile, error) {
	if a.profiles == nil {
		return nil, fmt.Errorf("profile store unavailable")
	}
	return a.profiles.List()
}

// ReadProfileContent returns raw string of the profile configuration.
func (a *App) ReadProfileContent(id string) (string, error) {
	if a.profiles == nil {
		return "", fmt.Errorf("profile store unavailable")
	}
	return a.profiles.ReadContent(id)
}

// SaveProfileContent updates raw string of the profile configuration.
func (a *App) SaveProfileContent(id string, content string) error {
	if a.profiles == nil {
		return fmt.Errorf("profile store unavailable")
	}
	return a.profiles.WriteContent(id, content)
}

// GetGlobalSettings returns the global settings configuration.
func (a *App) GetGlobalSettings() (client.GlobalSettings, error) {
	if a.settings == nil {
		return client.GlobalSettings{}, fmt.Errorf("settings store unavailable")
	}
	return a.settings.Load()
}

// SaveGlobalSettings updates the global settings configuration.
func (a *App) SaveGlobalSettings(s client.GlobalSettings) error {
	if a.settings == nil {
		return fmt.Errorf("settings store unavailable")
	}
	return a.settings.Save(s)
}

// PingAll measures TCP latency to all saved profiles.
func (a *App) PingAll() map[string]int {
	if a.profiles == nil {
		return nil
	}
	list, err := a.profiles.List()
	if err != nil {
		return nil
	}

	res := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, p := range list {
		wg.Add(1)
		go func(profile client.Profile) {
			defer wg.Done()
			addr := profile.Endpoint
			if addr == "" {
				return
			}
			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			lat := -1
			if err == nil {
				conn.Close()
				lat = int(time.Since(start).Milliseconds())
			}
			mu.Lock()
			res[profile.ID] = lat
			mu.Unlock()
		}(p)
	}
	wg.Wait()
	return res
}

// ProfilesDir — путь к директории профилей (для подсказки в UI).
func (a *App) ProfilesDir() string {
	if a.profiles == nil {
		return ""
	}
	return a.profiles.Dir()
}

// ImportProfile — открывает диалог выбора файла и импортирует выбранный .toml.
// Возвращает импортированный профиль или ошибку.
func (a *App) ImportProfile() (*client.Profile, error) {
	if a.profiles == nil {
		return nil, fmt.Errorf("profile store unavailable")
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Choose TrustTunnel profile",
		Filters: []wruntime.FileFilter{
			{DisplayName: "TrustTunnel profile (*.toml)", Pattern: "*.toml"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // user cancelled — frontend трактует nil как no-op
	}
	return a.profiles.Import(path)
}

// ImportProfileContent — для drop через Wails или paste content из textarea.
func (a *App) ImportProfileContent(content string, filename string) (*client.Profile, error) {
	if a.profiles == nil {
		return nil, fmt.Errorf("profile store unavailable")
	}
	if filename == "" {
		filename = "imported.toml"
	}
	return a.profiles.ImportContent([]byte(content), filename)
}

// DeleteProfile удаляет профиль по id.
func (a *App) DeleteProfile(id string) error {
	if a.profiles == nil {
		return fmt.Errorf("profile store unavailable")
	}
	if a.activeProfileID == id {
		_ = a.client.Disconnect()
		a.activeProfileID = ""
	}
	return a.profiles.Delete(id)
}

// OpenProfilesDir показывает директорию профилей в Finder/Explorer.
func (a *App) OpenProfilesDir() error {
	if a.profiles == nil {
		return fmt.Errorf("profile store unavailable")
	}
	wruntime.BrowserOpenURL(a.ctx, "file://"+a.profiles.Dir())
	return nil
}

// EnrollDevice привязывает устройство по ссылке из ЛК.
func (a *App) EnrollDevice(enrollURL string) (*client.EnrollResult, error) {
	if a.profiles == nil {
		return nil, fmt.Errorf("profile store unavailable")
	}
	enrollURL = strings.TrimSpace(enrollURL)
	if enrollURL == "" {
		return nil, fmt.Errorf("empty enroll URL")
	}
	result, err := client.Enroll(enrollURL, a.profiles)
	if err != nil {
		return nil, err
	}
	if result.OK && result.ProfileID != "" {
		_ = beeep.Notify("FireTunnel", "Устройство привязано", "")
	}
	if result.Revoked {
		_ = beeep.Notify("FireTunnel", "Устройство отозвано: "+result.Message, "")
	}
	return result, nil
}

// GetEnrollments returns all active enrollment states.
func (a *App) GetEnrollments() []client.EnrollState {
	return client.LoadAllEnrollStates()
}

// GetPendingDeepLink returns and clears the pending enroll URL from deep link.
func (a *App) GetPendingDeepLink() string {
	link := a.pendingDeepLink
	a.pendingDeepLink = ""
	return link
}

// HandleURLOpen processes a deep link URL received while the app is running.
func (a *App) HandleURLOpen(rawURL string) {
	slog.Info("deep link received while running", "url_prefix", rawURL[:min(len(rawURL), 20)]+"...")
	parsed := parseDeepLink([]string{"", rawURL})
	if parsed != "" && a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "deeplink", parsed)
	}
}

// GetConnections returns active + history TCP tunnels.
func (a *App) GetConnections() []client.ConnEntry {
	return a.client.AllConnections()
}

// AddExclusion adds a domain to global exclusions and saves settings.
func (a *App) AddExclusion(domain string) error {
	if a.settings == nil {
		return fmt.Errorf("settings store unavailable")
	}
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return fmt.Errorf("empty domain")
	}
	s, err := a.settings.Load()
	if err != nil {
		return err
	}
	for _, e := range s.GlobalExclusions {
		if e == domain {
			return nil
		}
	}
	s.GlobalExclusions = append(s.GlobalExclusions, domain)
	s.BypassDomains = true
	return a.settings.Save(s)
}

// ReadClipboard reads the system clipboard text (workaround for root losing clipboard access).
func (a *App) ReadClipboard() string {
	return readSystemClipboard()
}

// ReadLogs returns the last N lines of the application log file.
func (a *App) ReadLogs(lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	return readLogTail(lines)
}

// ImportProfileFromText imports a profile from raw TOML text.
func (a *App) ImportProfileFromText(content string, name string) (*client.Profile, error) {
	if a.profiles == nil {
		return nil, fmt.Errorf("profile store unavailable")
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("empty config")
	}
	if name == "" {
		name = "imported.toml"
	}
	if !strings.HasSuffix(name, ".toml") {
		name += ".toml"
	}
	return a.profiles.ImportContent([]byte(content), name)
}
