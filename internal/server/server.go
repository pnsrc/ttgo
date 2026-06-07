package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/http2"
	"github.com/quic-go/quic-go/http3"

	"github.com/pnsrc/ttgo/internal/config"
)

type Server struct {
	cfg       *config.Config
	hostsPath string
	tlsMgr    *TLSManager
	mainMux   *http.ServeMux // основной mux: VPN + ping
	vpnHandler http.Handler  // только VPN трафик
	h2srv     *http.Server
	h3srv     *http3.Server
}

func New(cfg *config.Config, hostsPath string, vpnHandler http.Handler) (*Server, error) {
	tlsMgr := NewTLSManager()
	hosts, err := config.LoadHosts(hostsPath)
	if err != nil {
		return nil, err
	}
	if err := tlsMgr.Load(hosts.MainHosts); err != nil {
		return nil, err
	}

	s := &Server{
		cfg:        cfg,
		hostsPath:  hostsPath,
		tlsMgr:     tlsMgr,
		vpnHandler: vpnHandler,
	}
	s.mainMux = s.buildMux(hosts, vpnHandler)
	return s, nil
}

// buildMux собирает финальный http.ServeMux с ping_hosts и основным handler.
func (s *Server) buildMux(hosts *config.HostsConfig, vpn http.Handler) *http.ServeMux {
	mux := http.NewServeMux()

	// Ping hosts: GET /ping → 200 OK (или любой путь на ping hostname)
	pingSet := make(map[string]bool)
	for _, h := range hosts.PingHosts {
		pingSet[h.Hostname] = true
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if idx := len(host); idx > 0 {
			// strip port
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
		}
		if pingSet[host] && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		vpn.ServeHTTP(w, r)
	})

	return mux
}

func (s *Server) Run(ctx context.Context) error {
	// Определяем порт для Alt-Svc
	_, port, _ := net.SplitHostPort(s.cfg.ListenAddress)
	if port == "" {
		port = "443"
	}

	var handler http.Handler = s.mainMux
	if s.cfg.ListenProtocols.QUIC != nil {
		handler = WrapHandler(handler, ":"+port)
	}

	tlsCfg := s.buildTLSConfig()

	if err := s.startHTTP2(tlsCfg, handler); err != nil {
		return err
	}
	if s.cfg.ListenProtocols.QUIC != nil {
		if err := s.startHTTP3(tlsCfg, handler); err != nil {
			slog.Warn("HTTP/3 unavailable", "err", err)
		}
	}

	go s.sighupLoop()

	slog.Info("endpoint running",
		"addr", s.cfg.ListenAddress,
		"http3", s.h3srv != nil,
	)

	<-ctx.Done()
	return s.shutdown()
}

func (s *Server) buildTLSConfig() *tls.Config {
	nextProtos := []string{"h2", "http/1.1"}
	if s.cfg.ListenProtocols.QUIC != nil {
		nextProtos = append([]string{"h3"}, nextProtos...)
	}
	return &tls.Config{
		GetCertificate: s.tlsMgr.GetCertificate,
		MinVersion:     tls.VersionTLS12,
		NextProtos:     nextProtos,
	}
}

func (s *Server) startHTTP2(tlsCfg *tls.Config, handler http.Handler) error {
	h2cfg := s.cfg.ListenProtocols.HTTP2
	h2opts := &http2.Server{
		MaxUploadBufferPerStream: 131072,
		MaxConcurrentStreams:     1000,
	}
	if h2cfg != nil {
		if h2cfg.InitialStreamWindowSize > 0 {
			h2opts.MaxUploadBufferPerStream = int32(h2cfg.InitialStreamWindowSize)
		}
		if h2cfg.MaxConcurrentStreams > 0 {
			h2opts.MaxConcurrentStreams = uint32(h2cfg.MaxConcurrentStreams)
		}
		if h2cfg.MaxFrameSize > 0 {
			h2opts.MaxReadFrameSize = uint32(h2cfg.MaxFrameSize)
		}
	}

	srv := &http.Server{
		Addr:        s.cfg.ListenAddress,
		Handler:     handler,
		TLSConfig:   tlsCfg,
		ConnContext: ConnContextFunc(),
		IdleTimeout: time.Duration(s.cfg.ClientListenerTimeoutSecs) * time.Second,
	}
	if err := http2.ConfigureServer(srv, h2opts); err != nil {
		return fmt.Errorf("configure http2: %w", err)
	}
	s.h2srv = srv

	// Создаём listener вручную чтобы вставить randomListener
	ln, err := net.Listen("tcp", s.cfg.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	// Оборачиваем: сначала peekRandom, потом TLS
	rl := NewRandomListener(ln)
	tlsLn := tls.NewListener(rl, tlsCfg)

	go func() {
		if err := srv.Serve(tlsLn); err != nil && err != http.ErrServerClosed {
			slog.Error("http2 server", "err", err)
			os.Exit(1)
		}
	}()
	return nil
}

func (s *Server) startHTTP3(tlsCfg *tls.Config, handler http.Handler) error {
	h3srv := &http3.Server{
		Addr:       s.cfg.ListenAddress,
		Handler:    handler,
		TLSConfig:  tlsCfg,
		QUICConfig: buildQUICConfig(s.cfg.ListenProtocols.QUIC),
	}
	s.h3srv = h3srv
	go func() {
		if err := h3srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("http3 server", "err", err)
		}
	}()
	return nil
}

func (s *Server) sighupLoop() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	for range ch {
		slog.Info("SIGHUP — reloading TLS hosts")
		hosts, err := config.LoadHosts(s.hostsPath)
		if err != nil {
			slog.Error("reload hosts", "err", err)
			continue
		}
		if err := s.tlsMgr.Load(hosts.MainHosts); err != nil {
			slog.Error("reload TLS certs", "err", err)
			continue
		}
		s.mainMux = s.buildMux(hosts, s.vpnHandler)
		s.h2srv.Handler = s.mainMux
		slog.Info("TLS hosts reloaded")
	}
}

func (s *Server) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.h3srv != nil {
		_ = s.h3srv.Close()
	}
	return s.h2srv.Shutdown(ctx)
}

// IsPrivateIP — RFC1918 / loopback / link-local.
func IsPrivateIP(ip net.IP) bool {
	for _, block := range privateBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

var privateBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
		"127.0.0.0/8", "::1/128", "fc00::/7", "fe80::/10", "169.254.0.0/16",
	} {
		_, block, _ := net.ParseCIDR(cidr)
		privateBlocks = append(privateBlocks, block)
	}
}
