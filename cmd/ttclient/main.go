package main

import (
	"embed"
	"io"
	"log"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logFile := setupLogging()
	if logFile != nil {
		defer logFile.Close()
	}

	slog.Info("ttclient starting", "os", runtime.GOOS, "arch", runtime.GOARCH, "args", os.Args)
	ensureElevated()

	app := NewApp()
	if link := parseDeepLink(os.Args); link != "" {
		app.pendingDeepLink = link
	}
	if fileLink := readAndClearDeepLinkFile(); fileLink != "" && app.pendingDeepLink == "" {
		app.pendingDeepLink = fileLink
		slog.Info("deep link loaded from file")
	}

	err := wails.Run(&options.App{
		Title:             "FireTunnel",
		Width:             420,
		Height:            620,
		MinWidth:          360,
		MinHeight:         560,
		DisableResize:     false,
		Frameless:         runtime.GOOS == "windows",
		HideWindowOnClose: true,
		BackgroundColour:  &options.RGBA{R: 10, G: 10, B: 10, A: 1},

		AssetServer: &assetserver.Options{Assets: assets},

		OnStartup:  app.startup,
		OnDomReady: app.domReady,
		OnShutdown: app.shutdown,

		Bind: []interface{}{app},

		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			DisableWindowIcon:                 false,
			Theme:                             windows.Dark,
			DisableFramelessWindowDecorations: false,
		},

		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "FireTunnel",
				Message: "Desktop client for TrustTunnel VPN",
			},
			OnUrlOpen: func(url string) {
				app.HandleURLOpen(url)
			},
		},
	})

	if err != nil {
		slog.Error("wails exited with error", "err", err)
		log.Fatalln("wails:", err)
	}
}

func setupLogging() *os.File {
	var logDir string
	if runtime.GOOS == "windows" {
		logDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "ttclient")
	} else {
		home, _ := os.UserHomeDir()
		logDir = filepath.Join(home, ".config", "ttclient")
	}
	os.MkdirAll(logDir, 0755)

	logPath := filepath.Join(logDir, "ttclient.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))
		slog.Warn("could not open log file, logging to stdout only", "path", logPath, "err", err)
		return nil
	}

	multi := io.MultiWriter(os.Stdout, f)
	slog.SetDefault(slog.New(slog.NewTextHandler(multi, &slog.HandlerOptions{Level: slog.LevelDebug})))
	slog.Info("log file opened", "path", logPath)
	return f
}

func parseDeepLink(args []string) string {
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "firetunnel://") {
			u, err := url.Parse(arg)
			if err != nil {
				continue
			}
			if u.Host == "enroll" {
				enrollURL := u.Query().Get("url")
				if enrollURL != "" {
					slog.Info("deep link enroll detected", "url_prefix", enrollURL[:min(len(enrollURL), 20)]+"...")
					return enrollURL
				}
			}
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
