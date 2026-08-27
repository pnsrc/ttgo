package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pnsrc/ttgo/internal/auth"
	icmpmux "github.com/pnsrc/ttgo/internal/icmp"
	"github.com/pnsrc/ttgo/internal/rules"
	"github.com/pnsrc/ttgo/internal/server"
	udpmux "github.com/pnsrc/ttgo/internal/udp"
)

const (
	pseudoHostUDP   = "_udp2"
	pseudoHostICMP  = "_icmp"
	pseudoHostCheck = "_check"
	pseudoHostP2P   = "_p2p"
)

type Options struct {
	AllowPrivateNet    bool
	ConnectTimeoutSecs int
	TCPIdleTimeoutSecs int
	AuthFailureCode    int
}

type Handler struct {
	auth    *auth.Authenticator
	rules   *rules.Engine
	opts    Options
	dialer  *net.Dialer
	udpMux  *udpmux.Mux
	icmpMux *icmpmux.Mux
	p2p     *P2PBroker
}

func New(a *auth.Authenticator, re *rules.Engine, opts Options, udp *udpmux.Mux, icmp *icmpmux.Mux) *Handler {
	timeout := time.Duration(opts.ConnectTimeoutSecs) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Handler{
		auth:  a,
		rules: re,
		opts:  opts,
		dialer: &net.Dialer{
			Timeout:   timeout,
			KeepAlive: 30 * time.Second,
		},
		udpMux:  udp,
		icmpMux: icmp,
		p2p:     NewP2PBroker(),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodConnect {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	rawConn := server.RawConnFromContext(r.Context())

	// Revocation check — до auth, чтобы клиент увидел 407 с причиной
	// вместо просто разрыва соединения.
	if revoked, reason := server.GlobalConnTracker.IsRevoked(rawConn); revoked {
		w.Header().Set("Proxy-Authenticate", `Basic realm="TrustTunnel"`)
		w.Header().Set("X-Revoke-Reason", reason)
		http.Error(w, "Session revoked: "+reason, http.StatusProxyAuthRequired)
		return
	}

	// Auth (расширенный — username + device limit + lifecycle)
	authRes := h.auth.CheckExt(r)
	username := authRes.Username

	// Lifecycle ошибка (юзер существует, но отключён/просрочен/над лимитом) —
	// возвращаем 407 с понятной причиной.
	if username != "" && authRes.Deny != auth.DenyNone {
		w.Header().Set("Proxy-Authenticate", `Basic realm="TrustTunnel"`)
		w.Header().Set("X-Revoke-Reason", string(authRes.Deny))
		http.Error(w, string(authRes.Deny), http.StatusProxyAuthRequired)
		return
	}

	if username == "" {
		if h.opts.AuthFailureCode == 405 {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		} else {
			w.Header().Set("Proxy-Authenticate", `Basic realm="TrustTunnel"`)
			http.Error(w, "Proxy Authentication Required", http.StatusProxyAuthRequired)
		}
		return
	}

	// Device limit: проверяем что у юзера не больше maxDevices активных соединений.
	// 0 = без ограничений. Проверяем только если это новое соединение.
	if authRes.MaxDevices > 0 && rawConn != nil {
		if !server.GlobalConnTracker.IsKnown(rawConn) {
			active := server.GlobalConnTracker.CountUser(username)
			if active >= authRes.MaxDevices {
				w.Header().Set("Proxy-Authenticate", `Basic realm="TrustTunnel"`)
				w.Header().Set("X-Revoke-Reason", "Device limit reached")
				http.Error(w,
					fmt.Sprintf("Device limit reached: %d/%d connections in use", active, authRes.MaxDevices),
					http.StatusProxyAuthRequired)
				return
			}
		}
	}

	// Регистрируем соединение в ConnTracker.
	if rawConn != nil {
		server.GlobalConnTracker.Register(username, rawConn)
		defer server.GlobalConnTracker.Unregister(username, rawConn)
	}

	// Rules: IP + TLS client random из ClientHello
	clientIP := remoteIP(r)
	clientRandom := server.TLSRandomFromContext(r.Context())
	if !h.rules.Allow(clientIP, clientRandom) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	target := r.Host
	if target == "" {
		target = r.URL.Host
	}

	switch target {
	case pseudoHostCheck:
		w.WriteHeader(http.StatusOK)

	case pseudoHostUDP:
		if h.udpMux == nil {
			http.Error(w, "UDP not available", http.StatusServiceUnavailable)
			return
		}
		// Per-session: UDP tracker живёт пока живёт TLS соединение,
		// cleanup через ctx.Done() в session.go
		sess := server.SessionFromContext(r.Context())
		if sess == nil {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		sess.UDP(h.udpMux).ServeHTTP(w, r)

	case pseudoHostICMP:
		if h.icmpMux == nil {
			http.Error(w, "ICMP not available", http.StatusServiceUnavailable)
			return
		}
		sess := server.SessionFromContext(r.Context())
		if sess == nil {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		sess.ICMP(h.icmpMux).ServeHTTP(w, r)

	case pseudoHostP2P:
		// P2P relay между двумя устройствами одного юзера в одной комнате.
		h.p2p.Handle(w, r, username)

	default:
		h.handleTCPTunnel(w, r, target, username)
	}
}

func (h *Handler) handleTCPTunnel(w http.ResponseWriter, r *http.Request, target, username string) {
	rawConn := server.RawConnFromContext(r.Context())
	if !h.opts.AllowPrivateNet {
		host, _, _ := net.SplitHostPort(target)
		if host == "" {
			host = target
		}
		if ip := resolveIP(r.Context(), host); ip != nil && server.IsPrivateIP(ip) {
			slog.Warn("blocked private target", "target", target, "user", username)
			http.Error(w, fmt.Sprintf("%s is a private address", target), http.StatusForbidden)
			return
		}
	}

	// HTTP/1.1: нужен hijack для bidirectional streaming
	if isHTTP1(r) {
		serveHTTP1Tunnel(w, r, target, username, h.dialer)
		return
	}

	// HTTP/2 и HTTP/3: ResponseWriter поддерживает streaming напрямую
	conn, err := h.dialer.DialContext(r.Context(), "tcp", target)
	if err != nil {
		slog.Warn("dial failed", "target", target, "user", username, "err", err)
		http.Error(w, fmt.Sprintf("cannot connect to %s", target), http.StatusBadGateway)
		return
	}
	defer conn.Close()

	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	slog.Debug("tunnel open", "target", target, "user", username)
	server.GlobalConnTracker.TunnelOpened(rawConn)
	defer server.GlobalConnTracker.TunnelClosed(rawConn)

	errCh := make(chan error, 2)
	go func() {
		n, err := io.Copy(conn, r.Body)
		server.GlobalConnTracker.AddBytes(rawConn, uint64(n), 0) // client → us
		errCh <- err
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()
	go func() {
		n, err := copyFlush(w, conn)
		server.GlobalConnTracker.AddBytes(rawConn, 0, uint64(n)) // us → client
		errCh <- err
	}()

	// Ждём завершение обоих направлений, иначе режем ответный трафик слишком рано.
	for i := 0; i < 2; i++ {
		select {
		case <-r.Context().Done():
			return
		case <-errCh:
		}
	}
	slog.Debug("tunnel closed", "target", target, "user", username)
}

func copyFlush(w http.ResponseWriter, src io.Reader) (int64, error) {
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			wn, werr := w.Write(buf[:n])
			total += int64(wn)
			if canFlush {
				flusher.Flush()
			}
			if werr != nil {
				return total, werr
			}
		}
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
	}
}

func remoteIP(r *http.Request) net.IP {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := net.ParseIP(strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])); ip != nil {
			return ip
		}
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return net.ParseIP(host)
}

func resolveIP(ctx context.Context, host string) net.IP {
	if ip := net.ParseIP(host); ip != nil {
		return ip
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil
	}
	return ips[0]
}
