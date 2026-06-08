package admin

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// TunnelConfig — общие настройки для expose и reach.
type TunnelConfig struct {
	Server   string // endpoint host:port (например, "example.com:443")
	Hostname string // SNI / Host header (если отличается от server)
	Username string
	Password string
	Room     string
	Insecure bool // skip TLS verification

	// expose mode
	Target string // куда диалить на этой машине, e.g. "127.0.0.1:22"
	Pool   int    // сколько pending listen-сессий держать одновременно

	// reach mode
	Local string // что слушать локально, e.g. "127.0.0.1:2222"
}

// p2pStatusWaiting / p2pStatusConnected — значения заголовка X-P2P-Status.
const (
	p2pHeaderRoom   = "X-P2P-Room"
	p2pHeaderTarget = "X-P2P-Target" // expose сообщает reach куда диалить локально
)

// newHTTP2Client делает HTTP/2 клиента для CONNECT-туннелей.
func newHTTP2Client(cfg TunnelConfig) *http.Client {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.Insecure,
		NextProtos:         []string{"h2"},
	}
	if cfg.Hostname != "" {
		tlsCfg.ServerName = cfg.Hostname
	}
	transport := &http2.Transport{
		TLSClientConfig: tlsCfg,
		AllowHTTP:       false,
	}
	return &http.Client{Transport: transport, Timeout: 0}
}

// p2pConnect открывает один CONNECT _p2p запрос и возвращает duplex stream.
// При expose — этот стрим оживает когда reach соединится с тем же room.
// При reach — этот стрим сразу пытается связаться с ожидающим expose.
func p2pConnect(ctx context.Context, client *http.Client, cfg TunnelConfig) (io.ReadWriteCloser, error) {
	pr, pw := io.Pipe()
	url := "https://" + cfg.Server + "/"

	req, err := http.NewRequestWithContext(ctx, http.MethodConnect, url, pr)
	if err != nil {
		return nil, err
	}
	req.Host = "_p2p"
	req.URL.Host = "_p2p"

	auth := base64.StdEncoding.EncodeToString([]byte(cfg.Username + ":" + cfg.Password))
	req.Header.Set("Proxy-Authorization", "Basic "+auth)
	req.Header.Set(p2pHeaderRoom, cfg.Room)

	resp, err := client.Do(req)
	if err != nil {
		pw.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		pw.Close()
		return nil, fmt.Errorf("server returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return &rwcCombined{Reader: resp.Body, Writer: pw, closers: []io.Closer{resp.Body, pw}}, nil
}

// rwcCombined объединяет ReadCloser и WriteCloser в один ReadWriteCloser.
type rwcCombined struct {
	io.Reader
	io.Writer
	closers []io.Closer
	once    sync.Once
}

func (r *rwcCombined) Close() error {
	var err error
	r.once.Do(func() {
		for _, c := range r.closers {
			if e := c.Close(); e != nil && err == nil {
				err = e
			}
		}
	})
	return err
}

// RunExpose запускает expose-режим: для каждого подключения партнёра
// открывает TCP к Target и проксит данные.
func RunExpose(ctx context.Context, cfg TunnelConfig, log func(string)) error {
	if cfg.Target == "" {
		return fmt.Errorf("--target required")
	}
	if cfg.Pool < 1 {
		cfg.Pool = 4
	}
	client := newHTTP2Client(cfg)

	log(fmt.Sprintf("expose: server=%s room=%q target=%s pool=%d",
		cfg.Server, cfg.Room, cfg.Target, cfg.Pool))

	sem := make(chan struct{}, cfg.Pool)
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			stream, err := p2pConnect(ctx, client, cfg)
			if err != nil {
				log("expose: " + err.Error())
				time.Sleep(2 * time.Second)
				return
			}
			defer stream.Close()

			// Партнёр пришёл — диалим target и стримим
			target, err := net.Dial("tcp", cfg.Target)
			if err != nil {
				log("expose: dial " + cfg.Target + ": " + err.Error())
				return
			}
			defer target.Close()

			log("expose: peer connected → " + cfg.Target)
			pipe(stream, target)
		}()
	}
}

// RunReach запускает reach-режим: открывает Local listener,
// для каждого incoming TCP создаёт P2P-стрим к экспозеру.
func RunReach(ctx context.Context, cfg TunnelConfig, log func(string)) error {
	if cfg.Local == "" {
		return fmt.Errorf("--local required")
	}
	client := newHTTP2Client(cfg)

	ln, err := net.Listen("tcp", cfg.Local)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Local, err)
	}
	defer ln.Close()
	log(fmt.Sprintf("reach: listening on %s, forwarding to room %q via %s",
		cfg.Local, cfg.Room, cfg.Server))

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}

		go func(local net.Conn) {
			defer local.Close()

			stream, err := p2pConnect(ctx, client, cfg)
			if err != nil {
				log("reach: " + err.Error())
				return
			}
			defer stream.Close()

			log("reach: " + local.RemoteAddr().String() + " ↔ peer")
			pipe(stream, local)
		}(c)
	}
}

// pipe — bidirectional copy с закрытием при разрыве любой стороны.
func pipe(a, b io.ReadWriteCloser) {
	done := make(chan struct{}, 2)
	go func() { io.Copy(a, b); done <- struct{}{} }()
	go func() { io.Copy(b, a); done <- struct{}{} }()
	<-done
}
