package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/pnsrc/ttgo/internal/client"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App — Wails-bound объект. Все экспортируемые методы доступны из frontend
// через wails-сгенерированные TS bindings (frontend/wailsjs/...).
type App struct {
	ctx      context.Context
	client   *client.Client
	profiles *client.ProfileStore

	activeProfileID string // профиль с которым последний раз делали Connect
}

func NewApp() *App {
	store, err := client.NewProfileStore()
	if err != nil {
		slog.Warn("profile store init", "err", err)
	}
	return &App{
		client:   client.New(),
		profiles: store,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) domReady(_ context.Context) {
	go a.pushLoop()
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
	a.activeProfileID = ""
	return a.client.Connect(cfg)
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
	a.activeProfileID = id
	if err := a.client.Connect(p.ToConfig()); err != nil {
		a.activeProfileID = ""
		return err
	}
	return nil
}

func (a *App) Disconnect() error {
	a.activeProfileID = ""
	return a.client.Disconnect()
}

// Profiles — список всех сохранённых профилей.
func (a *App) Profiles() ([]client.Profile, error) {
	if a.profiles == nil {
		return nil, fmt.Errorf("profile store unavailable")
	}
	return a.profiles.List()
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

// suppress unused import lint when os only used conditionally
var _ = os.Getenv
