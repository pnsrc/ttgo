package client

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// State — публичное состояние клиента для UI.
type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateError        State = "error"
)

// Snapshot — то что отдаём в UI.
type Snapshot struct {
	State      State  `json:"state"`
	Error      string `json:"error,omitempty"`
	TunName    string `json:"tun_name,omitempty"`
	Endpoint   string `json:"endpoint,omitempty"`
	BytesIn    uint64 `json:"bytes_in"`
	BytesOut   uint64 `json:"bytes_out"`
	Tunnels    int64  `json:"tunnels"`
	ActiveConn int32  `json:"active_conn"`
	UptimeSec  int64  `json:"uptime_sec"`
}

// Client связывает Dialer + TunDevice + RouteManager и управляет
// lifecycle подключения.
type Client struct {
	mu       sync.Mutex
	state    atomic.Value // State
	errStr   atomic.Value // string
	openedAt atomic.Int64

	dialer *Dialer
	tun    *TunDevice
	routes *RouteManager
	stats  Stats

	cfg Config
	ctx context.Context
}

func New() *Client {
	c := &Client{ctx: context.Background()}
	c.state.Store(StateDisconnected)
	c.errStr.Store("")
	return c
}

// Connect устанавливает все компоненты по порядку.
// Если что-то падает — чистит за собой.
func (c *Client) Connect(cfg Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if s := c.state.Load().(State); s == StateConnected || s == StateConnecting {
		return fmt.Errorf("already %s", s)
	}

	c.setState(StateConnecting, "")
	c.cfg = cfg

	dialer, err := NewDialer(cfg, &c.stats)
	if err != nil {
		c.setState(StateError, err.Error())
		return err
	}

	// Проверяем auth до того как трогать TUN/routes.
	authCtx, cancel := context.WithTimeout(c.ctx, 15*time.Second)
	defer cancel()
	if err := dialer.CheckAuth(authCtx); err != nil {
		dialer.Close()
		c.setState(StateError, "auth: "+err.Error())
		return err
	}
	slog.Info("auth ok", "endpoint", cfg.Endpoint, "user", cfg.Username)

	// Поднимаем TUN.
	tunDev, err := OpenTUN(c.ctx, dialer, &c.stats)
	if err != nil {
		dialer.Close()
		c.setState(StateError, "tun: "+err.Error())
		return err
	}

	// Маршруты — split-default + endpoint bypass.
	routes, err := NewRouteManager(tunDev.Name(), cfg.Endpoint, cfg.Exclusions)
	if err != nil {
		tunDev.Close()
		dialer.Close()
		c.setState(StateError, "routes: "+err.Error())
		return err
	}
	if cfg.EnableAdBlock {
		tunDev.SetAdBlocker(NewAdBlocker())
	}
	if cfg.UpstreamDNS != "" {
		tunDev.SetUpstreamDNS(cfg.UpstreamDNS)
	}
	tunDev.SetDNSBypass(cfg.Exclusions, routes.AddDynamicHostBypassBatch)

	if err := routes.Install(); err != nil {
		tunDev.Close()
		dialer.Close()
		c.setState(StateError, "routes install: "+err.Error())
		return err
	}

	c.dialer = dialer
	c.tun = tunDev
	c.routes = routes
	c.openedAt.Store(time.Now().Unix())
	c.setState(StateConnected, "")
	slog.Info("connected", "tun", tunDev.Name())
	return nil
}

// Disconnect разрывает всё в обратном порядке.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state.Load().(State) == StateDisconnected {
		return nil
	}

	var errs []string
	if c.routes != nil {
		if err := c.routes.Restore(); err != nil {
			errs = append(errs, "routes: "+err.Error())
		}
		c.routes = nil
	}
	if c.tun != nil {
		if err := c.tun.Close(); err != nil {
			errs = append(errs, "tun: "+err.Error())
		}
		c.tun = nil
	}
	if c.dialer != nil {
		c.dialer.Close()
		c.dialer = nil
	}

	// Reset stats
	c.stats.BytesIn.Store(0)
	c.stats.BytesOut.Store(0)
	c.stats.Tunnels.Store(0)
	c.stats.ActiveConn.Store(0)
	c.openedAt.Store(0)

	c.setState(StateDisconnected, "")
	slog.Info("disconnected")

	if len(errs) > 0 {
		return fmt.Errorf("disconnect errors: %v", errs)
	}
	return nil
}

// Status — снимок для UI (вызывается часто).
func (c *Client) Status() Snapshot {
	s := Snapshot{
		State:      c.state.Load().(State),
		Error:      c.errStr.Load().(string),
		BytesIn:    c.stats.BytesIn.Load(),
		BytesOut:   c.stats.BytesOut.Load(),
		Tunnels:    c.stats.Tunnels.Load(),
		ActiveConn: c.stats.ActiveConn.Load(),
	}
	if t := c.openedAt.Load(); t > 0 {
		s.UptimeSec = time.Now().Unix() - t
	}
	if c.tun != nil {
		s.TunName = c.tun.Name()
	}
	s.Endpoint = c.cfg.Endpoint
	return s
}

func (c *Client) AllConnections() []ConnEntry {
	if c.tun == nil {
		return nil
	}
	return c.tun.AllConns()
}

func (c *Client) setState(s State, errMsg string) {
	c.state.Store(s)
	c.errStr.Store(errMsg)
}
