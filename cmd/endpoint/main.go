package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pnsrc/ttgo/internal/admin"
	"github.com/pnsrc/ttgo/internal/auth"
	"github.com/pnsrc/ttgo/internal/auth/store"
	"github.com/pnsrc/ttgo/internal/config"
	icmpmux "github.com/pnsrc/ttgo/internal/icmp"
	"github.com/pnsrc/ttgo/internal/proxy"
	"github.com/pnsrc/ttgo/internal/rules"
	"github.com/pnsrc/ttgo/internal/server"
	udpmux "github.com/pnsrc/ttgo/internal/udp"
)

// rulesEngAdapter оборачивает rules.Engine в server.RulesEngine.
type rulesEngAdapter struct{ e *rules.Engine }

func (a rulesEngAdapter) Snapshot() []server.RuleView {
	src := a.e.Snapshot()
	out := make([]server.RuleView, len(src))
	for i, r := range src {
		out[i] = server.RuleView{
			CIDR:               r.CIDR,
			ClientRandomPrefix: r.ClientRandomPrefix,
			Action:             r.Action,
		}
	}
	return out
}

func (a rulesEngAdapter) Set(views []server.RuleView) error {
	in := make([]rules.RuleView, len(views))
	for i, v := range views {
		in[i] = rules.RuleView{
			CIDR:               v.CIDR,
			ClientRandomPrefix: v.ClientRandomPrefix,
			Action:             v.Action,
		}
	}
	return a.e.Set(in)
}

func (a rulesEngAdapter) Path() string { return a.e.Path() }

// adminStoreAdapter оборачивает admin.UserStore в server.AdminUserStore.
type adminStoreAdapter struct{ s admin.UserStore }

func (a adminStoreAdapter) List() ([]server.AdminUser, error) {
	entries, err := a.s.List()
	if err != nil {
		return nil, err
	}
	out := make([]server.AdminUser, 0, len(entries))
	for _, e := range entries {
		out = append(out, server.AdminUser{
			Username:     e.Username,
			MaxDevices:   e.MaxDevices,
			Enabled:      e.Enabled,
			ExpiresAt:    e.ExpiresAt,
			TrafficLimit: e.TrafficLimit,
			TrafficUsed:  e.TrafficUsed,
		})
	}
	return out, nil
}
func (a adminStoreAdapter) Add(u, p string) error                  { return a.s.Add(u, p) }
func (a adminStoreAdapter) Delete(u string) error                  { return a.s.Delete(u) }
func (a adminStoreAdapter) ChangePassword(u, p string) error       { return a.s.ChangePassword(u, p) }
func (a adminStoreAdapter) SetMaxDevices(u string, n int) error    { return a.s.SetMaxDevices(u, n) }

// Lifecycle operations (если sqlite/postgres)
func (a adminStoreAdapter) SetEnabled(u string, en bool) error {
	if lc, ok := a.s.(admin.LifecycleAdmin); ok {
		return lc.SetEnabled(u, en)
	}
	return fmt.Errorf("not supported for %s store", a.s.StoreType())
}
func (a adminStoreAdapter) SetExpiresAt(u string, exp int64) error {
	if lc, ok := a.s.(admin.LifecycleAdmin); ok {
		return lc.SetExpiresAt(u, exp)
	}
	return fmt.Errorf("not supported for %s store", a.s.StoreType())
}
func (a adminStoreAdapter) SetTrafficLimit(u string, b uint64) error {
	if lc, ok := a.s.(admin.LifecycleAdmin); ok {
		return lc.SetTrafficLimit(u, b)
	}
	return fmt.Errorf("not supported for %s store", a.s.StoreType())
}
func (a adminStoreAdapter) ResetTraffic(u string) error {
	if lc, ok := a.s.(admin.LifecycleAdmin); ok {
		// Reset нужно сбросить также flushedBytes в ConnTracker чтобы дельты не задвоились
		server.GlobalConnTracker.ResetUserTraffic(u)
		return lc.ResetTraffic(u)
	}
	return fmt.Errorf("not supported for %s store", a.s.StoreType())
}

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

	// При инвалидации credentials — помечаем соединения как revoked (клиент получит
	// 407 с причиной на следующем запросе), затем через 2с GOAWAY + close.
	auth.OnInvalidate = func(username string) {
		server.GlobalConnTracker.KickUser(username, "Account suspended by administrator")
	}

	// Periodic traffic flush в persistent store (sqlite/postgres).
	// Каждые 30 секунд — дельта по каждому юзеру вливается в БД, переживает рестарты.
	if fl, ok := userStore.(server.TrafficFlusher); ok {
		server.GlobalConnTracker.StartTrafficFlusher(
			context.Background(), fl, 30*time.Second,
		)
		slog.Info("traffic flusher started", "interval", "30s")
	}

	// Admin API (опционально, только если настроен в конфиге).
	var adminStore server.AdminUserStore
	if cfg.Admin != nil && cfg.Admin.Token != "" {
		// Открываем admin store на основе vpn.toml (sqlite/file/postgres)
		if s, err := admin.OpenStoreFromVPN(admin.Paths{
			VPN:   vpnPath,
			Hosts: hostsPath,
			Creds: cfg.CredentialsFile,
		}); err == nil {
			adminStore = adminStoreAdapter{s: s}
		} else {
			slog.Warn("admin store unavailable", "err", err)
		}
	}
	webRoot := ""
	if cfg.Admin != nil {
		webRoot = cfg.Admin.WebRoot
	}

	// Полный snapshot конфига для /api/info
	fullCfg := buildFullConfig(cfg, hostsPath)

	// TLS hot reload — отправляем SIGHUP самим себе (server.go обрабатывает)
	tlsReload := func() error {
		p, err := os.FindProcess(os.Getpid())
		if err != nil {
			return err
		}
		return p.Signal(syscall.SIGHUP)
	}

	// patchConfigFn — мост: server-пакет вызывает наш writer из admin-пакета
	server.SetPatchConfigFn(func(path string, raw []byte) error {
		var ed admin.EditableConfig
		if err := admin.UnmarshalEditableConfig(raw, &ed); err != nil {
			return err
		}
		return admin.PatchVPNConfig(path, ed)
	})

	// Graceful restart через systemd: завершаем процесс, юнит с Restart=always
	// поднимет нас заново.
	restart := func() {
		slog.Info("admin: restart requested, exiting")
		os.Exit(0)
	}

	rulesEng := rules.New()
	if err := rulesEng.LoadFile(cfg.RulesFile); err != nil {
		slog.Error("load rules", "err", err)
		os.Exit(1)
	}

	server.StartAdminAPI(cfg.Admin, authn, adminStore, webRoot, fullCfg, tlsReload, rulesEngAdapter{rulesEng}, vpnPath, restart)

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

// buildFullConfig — snapshot конфига для /api/info.
func buildFullConfig(cfg *config.Config, hostsPath string) *server.FullConfig {
	out := &server.FullConfig{
		ListenAddress:    cfg.ListenAddress,
		StoreType:        cfg.StoreType,
		StoreDSN:         cfg.StoreDSN,
		CacheTTLSecs:     cfg.CacheTTLSecs,
		AllowPrivateNet:  cfg.AllowPrivateNet,
		IPv6Available:    cfg.IPv6Available,
		TCPIdleTimeout:   cfg.TCPConnectionsTimeoutSecs,
		UDPIdleTimeout:   cfg.UDPConnectionsTimeoutSecs,
		ConnectTimeout:   cfg.ConnectionEstablishTimeoutSecs,
		TLSHandshake:     cfg.TLSHandshakeTimeoutSecs,
		HTTP3Enabled:     cfg.ListenProtocols.QUIC != nil,
		ICMPEnabled:      cfg.ICMP != nil,
	}
	if hosts, err := config.LoadHosts(hostsPath); err == nil {
		for _, h := range hosts.MainHosts {
			info := server.HostInfo{Hostname: h.Hostname}
			if cert, err := server.ReadCertInfo(h.CertChainPath); err == nil {
				info.NotAfter = cert.NotAfter
				info.Issuer = cert.Issuer
			}
			out.Hostnames = append(out.Hostnames, info)
		}
	}
	return out
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
