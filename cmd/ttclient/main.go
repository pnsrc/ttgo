// ttclient — TrustTunnel десктопный клиент.
//
// Usage:
//
//	sudo ttclient            # запускает с GUI на http://127.0.0.1:7700
//	sudo ttclient -no-ui     # headless (CLI only)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/pnsrc/ttgo/internal/client"
)

func main() {
	uiAddr := flag.String("addr", "127.0.0.1:7700", "GUI HTTP address")
	noUI := flag.Bool("no-ui", false, "headless — don't open browser")
	autoOpen := flag.Bool("open", true, "open browser automatically")
	logLevel := flag.String("log", "info", "log level: debug|info|warn|error")
	flag.Parse()

	setupLogger(*logLevel)

	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "ttclient must run as root (TUN device requires elevated privileges)")
		fmt.Fprintln(os.Stderr, "Try: sudo ttclient")
		os.Exit(1)
	}

	c := client.New()

	mux := http.NewServeMux()
	registerAPI(mux, c)
	registerStatic(mux)

	srv := &http.Server{
		Addr:    *uiAddr,
		Handler: mux,
	}

	go func() {
		slog.Info("ui starting", "addr", *uiAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("ui server", "err", err)
		}
	}()

	if !*noUI && *autoOpen {
		go openBrowser("http://" + *uiAddr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	slog.Info("shutting down")
	c.Disconnect() // ensure routes/tun cleaned up

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

// ── API ──────────────────────────────────────────────────────────────────────

func registerAPI(mux *http.ServeMux, c *client.Client) {
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, c.Status())
	})

	mux.HandleFunc("/api/connect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		var cfg client.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := c.Connect(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, c.Status())
	})

	mux.HandleFunc("/api/disconnect", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		if err := c.Disconnect(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, c.Status())
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// ── static UI ────────────────────────────────────────────────────────────────

func registerStatic(mux *http.ServeMux) {
	// Пока что отдаём минимальный HTML inline. После того как
	// desktop/frontend будет готов — embed dist/ через embed.FS.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, fallbackHTML)
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	// Если запущено через sudo — open откроется в контексте root.
	// Делаем под оригинального юзера:
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && runtime.GOOS == "darwin" {
		cmd = exec.Command("sudo", "-u", sudoUser, "open", url)
	}
	_ = cmd.Start()
}

func setupLogger(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
}

const fallbackHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<title>TrustTunnel Client</title>
<meta name="viewport" content="width=device-width, initial-scale=1" />
<style>
  :root { color-scheme: dark; }
  body {
    background: #0a0a0a; color: #fafafa;
    font: 14px/1.4 -apple-system, system-ui, sans-serif;
    margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center;
  }
  .card {
    background: #111; border: 1px solid #262626; border-radius: 12px;
    padding: 32px; width: 100%; max-width: 380px;
  }
  h1 { margin: 0 0 4px; font-size: 18px; }
  .sub { color: #71717a; font-size: 12px; margin-bottom: 24px; }
  label { display: block; margin: 12px 0; font-size: 12px; color: #a1a1aa; }
  input {
    width: 100%; padding: 8px 12px; margin-top: 4px;
    background: #18181b; border: 1px solid #3f3f46; border-radius: 6px;
    color: #fafafa; font: inherit; box-sizing: border-box;
  }
  input:focus { outline: none; border-color: #71717a; }
  button {
    width: 100%; padding: 10px; margin-top: 16px;
    background: #fafafa; color: #0a0a0a; border: none; border-radius: 6px;
    font: 500 14px/1 inherit; cursor: pointer;
  }
  button:disabled { opacity: 0.4; cursor: not-allowed; }
  .danger { background: #ef4444; color: #fff; }
  .status {
    margin-top: 20px; padding: 12px; border-radius: 8px;
    background: #18181b; border: 1px solid #27272a;
    font-size: 12px; color: #a1a1aa;
  }
  .state { font-weight: 600; }
  .state.connected { color: #4ade80; }
  .state.connecting { color: #fbbf24; }
  .state.error { color: #f87171; }
  .row { display: flex; justify-content: space-between; margin-top: 6px; }
  .row span:last-child { color: #fafafa; font-variant-numeric: tabular-nums; }
  .err { color: #f87171; margin-top: 8px; word-break: break-all; }
  label.row-inline { display: flex; align-items: center; gap: 8px; }
  label.row-inline input { width: auto; margin: 0; }
</style>
</head>
<body>
  <div class="card">
    <h1>TrustTunnel</h1>
    <div class="sub">Desktop client</div>

    <label>Endpoint
      <input id="endpoint" placeholder="193-233-88-195.sslip.io:4883" />
    </label>
    <label>Hostname (SNI, optional)
      <input id="hostname" placeholder="leave empty to use endpoint host" />
    </label>
    <label>Username
      <input id="username" placeholder="user" />
    </label>
    <label>Password
      <input id="password" type="password" />
    </label>
    <label class="row-inline">
      <input id="insecure" type="checkbox" />
      <span>Skip TLS verification (self-signed)</span>
    </label>

    <button id="connect">Connect</button>
    <button id="disconnect" class="danger" style="display:none">Disconnect</button>

    <div class="status" id="status">
      <div class="state">disconnected</div>
    </div>
  </div>

<script>
const $ = id => document.getElementById(id);
const fmt = n => {
  if (n < 1024) return n + ' B';
  const u = ['KB','MB','GB','TB']; let i = 0; n /= 1024;
  while (n >= 1024 && i < u.length-1) { n /= 1024; i++; }
  return n.toFixed(n < 10 ? 2 : 1) + ' ' + u[i];
};
const fmtTime = s => {
  if (s < 60) return s + 's';
  if (s < 3600) return Math.floor(s/60) + 'm ' + (s%60) + 's';
  return Math.floor(s/3600) + 'h ' + Math.floor((s%3600)/60) + 'm';
};

// Восстанавливаем поля
['endpoint','hostname','username','insecure'].forEach(k => {
  const v = localStorage.getItem('tt_' + k);
  if (v !== null) {
    if (k === 'insecure') $(k).checked = v === '1';
    else $(k).value = v;
  }
});

async function connect() {
  ['endpoint','hostname','username'].forEach(k => localStorage.setItem('tt_'+k, $(k).value));
  localStorage.setItem('tt_insecure', $('insecure').checked ? '1' : '0');

  $('connect').disabled = true;
  const cfg = {
    endpoint: $('endpoint').value,
    hostname: $('hostname').value,
    username: $('username').value,
    password: $('password').value,
    insecure: $('insecure').checked,
  };
  try {
    const r = await fetch('/api/connect', {method:'POST', body: JSON.stringify(cfg)});
    if (!r.ok) throw new Error(await r.text());
  } catch (e) {
    alert('Connect failed: ' + e.message);
  }
  $('connect').disabled = false;
}

async function disconnect() {
  await fetch('/api/disconnect', {method:'POST'});
}

$('connect').onclick = connect;
$('disconnect').onclick = disconnect;

async function tick() {
  try {
    const r = await fetch('/api/status');
    const s = await r.json();
    const connected = s.state === 'connected';
    $('connect').style.display = connected ? 'none' : '';
    $('disconnect').style.display = connected ? '' : 'none';

    let html = '<div class="state ' + s.state + '">' + s.state + '</div>';
    if (s.error) html += '<div class="err">' + s.error + '</div>';
    if (connected) {
      html += '<div class="row"><span>Endpoint</span><span>' + (s.endpoint||'') + '</span></div>';
      html += '<div class="row"><span>TUN</span><span>' + (s.tun_name||'') + '</span></div>';
      html += '<div class="row"><span>Uptime</span><span>' + fmtTime(s.uptime_sec) + '</span></div>';
      html += '<div class="row"><span>Tunnels</span><span>' + s.active_conn + ' / ' + s.tunnels + '</span></div>';
      html += '<div class="row"><span>↓ In</span><span style="color:#4ade80">' + fmt(s.bytes_in) + '</span></div>';
      html += '<div class="row"><span>↑ Out</span><span style="color:#fbbf24">' + fmt(s.bytes_out) + '</span></div>';
    }
    $('status').innerHTML = html;
  } catch {}
}
tick();
setInterval(tick, 1000);
</script>
</body>
</html>`
