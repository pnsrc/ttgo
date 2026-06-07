package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pnsrc/ttgo/internal/auth"
	"github.com/pnsrc/ttgo/internal/auth/store"
	"github.com/pnsrc/ttgo/internal/config"
	icmpmux "github.com/pnsrc/ttgo/internal/icmp"
	"github.com/pnsrc/ttgo/internal/proxy"
	"github.com/pnsrc/ttgo/internal/rules"
	"github.com/pnsrc/ttgo/internal/server"
	udpmux "github.com/pnsrc/ttgo/internal/udp"
)

func main() {
	logLevel := flag.String("l", "info", "log level: info|debug|trace")
	logFile  := flag.String("logfile", "", "log file (default: stderr)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		slog.Error("usage: trusttunnel_endpoint [flags] <vpn.toml> <hosts.toml>")
		os.Exit(1)
	}
	vpnPath, hostsPath := args[0], args[1]

	setupLogger(*logLevel, *logFile)

	cfg, err := config.Load(vpnPath)
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	userStore, err := buildStore(cfg)
	if err != nil {
		slog.Error("init store", "err", err)
		os.Exit(1)
	}

	cacheTTL := time.Duration(cfg.CacheTTLSecs) * time.Second
	authn := auth.New(userStore, cacheTTL)

	// При инвалидации credentials — слать GOAWAY всем активным соединениям юзера.
	auth.OnInvalidate = func(username string) {
		server.GlobalConnTracker.KickUser(username)
	}

	// Admin API (опционально, только если настроен в конфиге).
	server.StartAdminAPI(cfg.Admin, authn)

	rulesEng := rules.New()
	if err := rulesEng.LoadFile(cfg.RulesFile); err != nil {
		slog.Error("load rules", "err", err)
		os.Exit(1)
	}

	udpMux := udpmux.NewMux(cfg.UDPConnectionsTimeoutSecs)

	var icmpHandler *icmpmux.Mux
	if cfg.ICMP != nil {
		im, err := icmpmux.NewMux(cfg.ICMP)
		if err != nil {
			slog.Warn("ICMP mux unavailable", "err", err)
		} else {
			icmpHandler = im
		}
	}

	proxyOpts := proxy.Options{
		AllowPrivateNet:    cfg.AllowPrivateNet,
		ConnectTimeoutSecs: cfg.ConnectionEstablishTimeoutSecs,
		TCPIdleTimeoutSecs: cfg.TCPConnectionsTimeoutSecs,
		AuthFailureCode:    cfg.AuthFailureStatusCode,
	}
	handler := proxy.New(authn, rulesEng, proxyOpts, udpMux, icmpHandler)

	srv, err := server.New(cfg, hostsPath, handler)
	if err != nil {
		slog.Error("init server", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		slog.Error("server", "err", err)
		os.Exit(1)
	}
}

func buildStore(cfg *config.Config) (auth.UserStore, error) {
	switch cfg.StoreType {
	case "sqlite":
		dsn := cfg.StoreDSN
		if dsn == "" {
			dsn = "users.db"
		}
		return store.NewSQLiteStore(dsn)
	case "postgres":
		return store.NewPostgresStore(cfg.StoreDSN)
	default: // "file"
		return store.NewFileStore(cfg.CredentialsFile), nil
	}
}

func setupLogger(level, file string) {
	var lvl slog.Level
	switch level {
	case "debug", "trace":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	out := os.Stderr
	if file != "" {
		f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			slog.Error("open log file", "err", err)
			os.Exit(1)
		}
		out = f
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: lvl})))
}
