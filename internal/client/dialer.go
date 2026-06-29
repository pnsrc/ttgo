// Package client implements the TrustTunnel desktop client side:
// HTTP/2 CONNECT dialing, TUN device handling, route management.
package client

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
)

// Config — все параметры подключения.
type Config struct {
	// "host:port" of TrustTunnel endpoint
	Endpoint string
	// SNI / Host header; if empty, derived from endpoint host.
	Hostname string
	Username string
	Password string

	// Skip cert verification (for self-signed certs)
	Insecure bool

	// Optional pinned PEM cert (overrides system roots if set)
	PinnedCertPEM []byte
}

// Stats — counters exposed to UI.
type Stats struct {
	BytesIn    atomic.Uint64
	BytesOut   atomic.Uint64
	Tunnels    atomic.Int64
	OpenedAt   atomic.Int64 // unix seconds
	ActiveConn atomic.Int32 // в данный момент открытых CONNECT-туннелей
}

// Dialer — long-lived HTTP/2 client to the endpoint. DialContext возвращает
// net.Conn для каждого CONNECT target. Используется как gvisor dialer.
type Dialer struct {
	cfg       Config
	tr        *http2.Transport
	httpc     *http.Client
	endpoint  *url.URL
	stats     *Stats
	authBasic string

	mu    sync.Mutex
	// reused TLS connection to endpoint — http2.Transport уже делает pooling
	// сам, но мы храним ссылку чтобы при Close прерывать всё.
	hostnameForSNI string
}

// NewDialer prepares a dialer. No network IO performed.
func NewDialer(cfg Config, stats *Stats) (*Dialer, error) {
	host, port, err := net.SplitHostPort(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("endpoint: %w", err)
	}
	sni := cfg.Hostname
	if sni == "" {
		sni = host
	}

	tlsCfg := &tls.Config{
		ServerName:         sni,
		InsecureSkipVerify: cfg.Insecure,
		NextProtos:         []string{"h2"},
		MinVersion:         tls.VersionTLS12,
	}
	if len(cfg.PinnedCertPEM) > 0 {
		pool := tlsPoolFromPEM(cfg.PinnedCertPEM)
		if pool != nil {
			tlsCfg.RootCAs = pool
		}
	}

	tr := &http2.Transport{
		TLSClientConfig: tlsCfg,
		// Force a single TLS connection per endpoint — нам не нужны мультиплексированные коннекты
		// для прокси: HTTP/2 уже мультиплексирует CONNECT в одном TCP.
		AllowHTTP:          false,
		DisableCompression: true,
		ReadIdleTimeout:    30 * time.Second,
		PingTimeout:        15 * time.Second,
	}

	httpc := &http.Client{
		Transport: tr,
		Timeout:   0, // CONNECT-туннели долгоживущие
	}

	endpoint := &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(host, port),
	}

	return &Dialer{
		cfg:            cfg,
		tr:             tr,
		httpc:          httpc,
		endpoint:       endpoint,
		stats:          stats,
		authBasic:      "Basic " + base64.StdEncoding.EncodeToString([]byte(cfg.Username+":"+cfg.Password)),
		hostnameForSNI: sni,
	}, nil
}

// DialContext opens a CONNECT tunnel to target ("host:port") and returns
// a bidirectional net.Conn for the byte stream. Не блокирует после возврата —
// конец туннеля живёт до Close или закрытия серверной стороны.
func (d *Dialer) DialContext(ctx context.Context, target string) (net.Conn, error) {
	pr, pw := io.Pipe() // body клиент → сервер (запись в pw → чтение сервером)
	req, err := http.NewRequestWithContext(ctx, http.MethodConnect, d.endpoint.String(), pr)
	if err != nil {
		pw.Close()
		return nil, err
	}
	// authority-form для CONNECT
	req.Host = target
	req.URL.Host = target
	req.URL.Path = ""
	req.Header.Set("Proxy-Authorization", d.authBasic)
	req.Header.Set("User-Agent", "ttgo-client/0.1")

	resp, err := d.httpc.Do(req)
	if err != nil {
		pw.Close()
		return nil, fmt.Errorf("connect %s: %w", target, err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		resp.Body.Close()
		pw.Close()
		return nil, fmt.Errorf("connect %s: %s: %s",
			target, resp.Status, strings.TrimSpace(string(body)))
	}

	d.stats.Tunnels.Add(1)
	d.stats.ActiveConn.Add(1)

	return newTunnelConn(target, resp.Body, pw, d.stats), nil
}

// Close tears down the underlying connection pool.
func (d *Dialer) Close() error {
	d.tr.CloseIdleConnections()
	return nil
}

// CheckAuth performs a CONNECT to a sentinel pseudo-host to verify credentials.
// Returns nil if auth succeeded.
func (d *Dialer) CheckAuth(ctx context.Context) error {
	conn, err := d.DialContext(ctx, "_check")
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// tunnelConn adapts an HTTP/2 response body + pipe writer into net.Conn.
type tunnelConn struct {
	target string
	reader io.ReadCloser // resp.Body — байты от сервера
	writer *io.PipeWriter
	stats  *Stats

	closeOnce sync.Once
	closed    chan struct{}

	// local "addresses" — gvisor требует чтобы LocalAddr/RemoteAddr возвращали что-то
	local  net.Addr
	remote net.Addr
}

func newTunnelConn(target string, r io.ReadCloser, w *io.PipeWriter, stats *Stats) *tunnelConn {
	return &tunnelConn{
		target: target,
		reader: r,
		writer: w,
		stats:  stats,
		closed: make(chan struct{}),
		local:  &addr{net: "ttgo", str: "client"},
		remote: &addr{net: "ttgo", str: target},
	}
}

func (c *tunnelConn) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	if n > 0 {
		c.stats.BytesIn.Add(uint64(n))
	}
	return n, err
}

func (c *tunnelConn) Write(p []byte) (int, error) {
	n, err := c.writer.Write(p)
	if n > 0 {
		c.stats.BytesOut.Add(uint64(n))
	}
	return n, err
}

func (c *tunnelConn) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
		c.writer.Close()
		c.reader.Close()
		c.stats.ActiveConn.Add(-1)
	})
	return nil
}

func (c *tunnelConn) LocalAddr() net.Addr  { return c.local }
func (c *tunnelConn) RemoteAddr() net.Addr { return c.remote }

// HTTP/2 stream не поддерживает deadline в любом понятном смысле;
// gvisor вызывает эти методы но обычно с нулевым deadline (без таймаута).
func (c *tunnelConn) SetDeadline(t time.Time) error      { return nil }
func (c *tunnelConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *tunnelConn) SetWriteDeadline(t time.Time) error { return nil }

type addr struct{ net, str string }

func (a *addr) Network() string { return a.net }
func (a *addr) String() string  { return a.str }
