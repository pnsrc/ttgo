package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RunWizard runs the interactive setup wizard.
func RunWizard(paths Paths) error {
	app := tview.NewApplication()

	pages := tview.NewPages()
	app.SetRoot(pages, true)

	var (
		hostname  string
		listenAddr string
		tlsType   int // 0=letsencrypt, 1=self-signed, 2=custom
		email     string
		certPath  string
		keyPath   string
		storeType int // 0=file, 1=sqlite, 2=postgres
		storeDSN  string
		username  string
		password  string
	)

	// ── helpers ──────────────────────────────────────────────────────────────

	header := func(title string) *tview.TextView {
		tv := tview.NewTextView().
			SetText(" TrustTunnel Setup Wizard — " + title).
			SetTextColor(tcell.ColorAqua)
		tv.SetBackgroundColor(tcell.ColorDarkSlateGray)
		return tv
	}

	hint := func(text string) *tview.TextView {
		tv := tview.NewTextView().SetText(text).SetTextColor(tcell.ColorGray)
		return tv
	}

	wrap := func(title string, content tview.Primitive) *tview.Flex {
		return tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(header(title), 1, 0, false).
			AddItem(tview.NewBox(), 1, 0, false).
			AddItem(content, 0, 1, true).
			AddItem(hint("  Tab/Enter to move  ·  Esc to go back  ·  Ctrl+C to quit"), 1, 0, false)
	}

	showErr := func(msg string) {
		modal := tview.NewModal().
			SetText("⚠ " + msg).
			AddButtons([]string{"OK"}).
			SetDoneFunc(func(_ int, _ string) {
				pages.RemovePage("error")
				app.SetFocus(pages)
			})
		pages.AddPage("error", modal, true, true)
	}

	// ── page 5: done ─────────────────────────────────────────────────────────

	showDone := func(summary string) {
		tv := tview.NewTextView().
			SetText(summary).
			SetDynamicColors(true).
			SetWordWrap(true)
		tv.SetBorder(true).SetTitle(" Setup Complete ")

		flex := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(header("Done!"), 1, 0, false).
			AddItem(tview.NewBox(), 1, 0, false).
			AddItem(tv, 0, 1, false).
			AddItem(tview.NewBox(), 1, 0, false).
			AddItem(tview.NewButton("[ Quit ]").SetSelectedFunc(func() {
				app.Stop()
			}), 1, 0, true)

		pages.AddPage("done", flex, true, true)
		app.SetFocus(flex)
	}

	applyConfig := func() {
		// Determine cert paths
		if tlsType == 0 || tlsType == 1 {
			base := filepath.Dir(paths.Hosts)
			certPath = filepath.Join(base, hostname+".crt")
			keyPath = filepath.Join(base, hostname+".key")
		}

		var summary strings.Builder
		summary.WriteString("[green]Generating TLS certificate...[white]\n\n")

		switch tlsType {
		case 0: // Let's Encrypt
			if err := ObtainCert(hostname, email, certPath, keyPath, func(s string) {
				summary.WriteString("  " + s + "\n")
			}); err != nil {
				showErr("Let's Encrypt failed: " + err.Error())
				return
			}
		case 1: // Self-signed
			if err := GenerateSelfSigned(hostname, certPath, keyPath); err != nil {
				showErr("Self-signed cert failed: " + err.Error())
				return
			}
			summary.WriteString("  Self-signed certificate generated\n")
		case 2: // Custom — paths already set
			if err := VerifyCert(certPath, keyPath); err != nil {
				showErr("Certificate verify failed: " + err.Error())
				return
			}
			summary.WriteString("  Custom certificate verified OK\n")
		}

		// Write credentials.toml
		summary.WriteString("\n[green]Writing credentials.toml...[white]\n")
		if err := saveCreds(paths.Creds, []credEntry{{Username: username, Password: password}}); err != nil {
			showErr("Write credentials: " + err.Error())
			return
		}

		// Write hosts.toml
		summary.WriteString("[green]Writing hosts.toml...[white]\n")
		hf := &hostsFile{
			MainHosts: []hostEntry{{
				Hostname:       hostname,
				CertChainPath:  certPath,
				PrivateKeyPath: keyPath,
			}},
			PingHosts: []hostEntry{{
				Hostname:       hostname,
				CertChainPath:  certPath,
				PrivateKeyPath: keyPath,
			}},
		}
		if err := saveHosts(paths.Hosts, hf); err != nil {
			showErr("Write hosts: " + err.Error())
			return
		}

		// Write vpn.toml
		summary.WriteString("[green]Writing vpn.toml...[white]\n")
		storeTypeName := []string{"file", "sqlite", "postgres"}[storeType]
		vpnContent := fmt.Sprintf(`listen_address = %q
ipv6_available = false
allow_private_network_connections = false

tls_handshake_timeout_secs = 10
client_listener_timeout_secs = 600
connection_establishment_timeout_secs = 30
tcp_connections_timeout_secs = 604800
udp_connections_timeout_secs = 300

credentials_file = %q
rules_file = ""

auth_failure_status_code = 407
store_type = %q
store_dsn = %q
cache_ttl_secs = 30

[listen_protocols.http2]
max_concurrent_streams = 1000
initial_stream_window_size = 131072
max_frame_size = 16384
`, listenAddr, paths.Creds, storeTypeName, storeDSN)

		if err := os.WriteFile(paths.VPN, []byte(vpnContent), 0644); err != nil {
			showErr("Write vpn.toml: " + err.Error())
			return
		}

		summary.WriteString("\n[yellow]Config files written:[white]\n")
		summary.WriteString(fmt.Sprintf("  %s\n  %s\n  %s\n", paths.VPN, paths.Hosts, paths.Creds))
		summary.WriteString("\n[green]Run the server:[white]\n")
		summary.WriteString(fmt.Sprintf("  trusttunnel_endpoint %s %s\n", paths.VPN, paths.Hosts))

		showDone(summary.String())
	}

	// ── page 4: first user ───────────────────────────────────────────────────

	var page4Form *tview.Form
	page4Form = tview.NewForm().
		AddInputField("Username", "admin", 30, nil, func(v string) { username = v }).
		AddPasswordField("Password", "", 30, '*', func(v string) { password = v }).
		AddButton("Apply →", func() {
			username = strings.TrimSpace(page4Form.GetFormItem(0).(*tview.InputField).GetText())
			password = page4Form.GetFormItem(1).(*tview.InputField).GetText()
			if username == "" || password == "" {
				showErr("Username and password are required")
				return
			}
			applyConfig()
		}).
		AddButton("← Back", func() { pages.SwitchToPage("page3") })
	page4Form.SetBorder(true).SetTitle(" Step 4/4: First User ")
	pages.AddPage("page4", wrap("First User", page4Form), true, false)

	// ── page 3a: custom cert paths ───────────────────────────────────────────

	var page3aForm *tview.Form
	page3aForm = tview.NewForm().
		AddInputField("Certificate (.crt / .pem)", "", 50, nil, func(v string) { certPath = v }).
		AddInputField("Private key (.key / .pem)", "", 50, nil, func(v string) { keyPath = v }).
		AddButton("Next →", func() {
			certPath = strings.TrimSpace(page3aForm.GetFormItem(0).(*tview.InputField).GetText())
			keyPath = strings.TrimSpace(page3aForm.GetFormItem(1).(*tview.InputField).GetText())
			if certPath == "" || keyPath == "" {
				showErr("Both paths are required")
				return
			}
			pages.SwitchToPage("page4")
		}).
		AddButton("← Back", func() { pages.SwitchToPage("page3") })
	page3aForm.SetBorder(true).SetTitle(" Step 3/4: Custom Certificate Paths ")
	pages.AddPage("page3a", wrap("Custom Certificate", page3aForm), true, false)

	// ── page 3: TLS type ─────────────────────────────────────────────────────

	var page3Form *tview.Form
	page3Form = tview.NewForm().
		AddDropDown("TLS type", []string{
			"Let's Encrypt (recommended)",
			"Self-signed (for IP or testing)",
			"Custom (I have my own cert)",
		}, 0, func(_ string, idx int) { tlsType = idx }).
		AddInputField("Email (for Let's Encrypt)", "", 40, nil, func(v string) { email = v }).
		AddButton("Next →", func() {
			email = strings.TrimSpace(page3Form.GetFormItem(1).(*tview.InputField).GetText())
			if tlsType == 0 && email == "" {
				showErr("Email is required for Let's Encrypt")
				return
			}
			if tlsType == 2 {
				pages.SwitchToPage("page3a")
			} else {
				pages.SwitchToPage("page4")
			}
		}).
		AddButton("← Back", func() { pages.SwitchToPage("page2") })
	page3Form.SetBorder(true).SetTitle(" Step 3/4: TLS Certificate ")
	pages.AddPage("page3", wrap("TLS Certificate", page3Form), true, false)

	// ── page 2: store + listen addr ──────────────────────────────────────────

	var page2Form *tview.Form
	page2Form = tview.NewForm().
		AddInputField("Listen address", "0.0.0.0:443", 25, nil, func(v string) { listenAddr = v }).
		AddDropDown("User store", []string{
			"file  (credentials.toml)",
			"sqlite  (local DB)",
			"postgres  (remote DB)",
		}, 0, func(_ string, idx int) { storeType = idx }).
		AddInputField("Store DSN (sqlite path / postgres url)", "", 50, nil, func(v string) { storeDSN = v }).
		AddButton("Next →", func() {
			listenAddr = strings.TrimSpace(page2Form.GetFormItem(0).(*tview.InputField).GetText())
			if listenAddr == "" {
				listenAddr = "0.0.0.0:443"
			}
			pages.SwitchToPage("page3")
		}).
		AddButton("← Back", func() { pages.SwitchToPage("page1") })
	page2Form.SetBorder(true).SetTitle(" Step 2/4: Server Settings ")
	pages.AddPage("page2", wrap("Server Settings", page2Form), true, false)

	// ── page 1: hostname ─────────────────────────────────────────────────────

	var page1Form *tview.Form
	page1Form = tview.NewForm().
		AddInputField("Server hostname or IP", "", 40, nil, func(v string) { hostname = v }).
		AddButton("Next →", func() {
			hostname = strings.TrimSpace(page1Form.GetFormItem(0).(*tview.InputField).GetText())
			if hostname == "" {
				showErr("Hostname is required")
				return
			}
			pages.SwitchToPage("page2")
		}).
		AddButton("Quit", func() { app.Stop() })
	page1Form.SetBorder(true).SetTitle(" Step 1/4: Hostname ")
	pages.AddPage("page1", wrap("Hostname / IP", page1Form), true, false)

	// ── welcome ───────────────────────────────────────────────────────────────

	welcomeText := tview.NewTextView().
		SetText(`
  Welcome to TrustTunnel Setup Wizard

  This wizard will help you configure:
    • Listen address and port
    • TLS certificate (Let's Encrypt, self-signed, or custom)
    • User store (file, SQLite, or PostgreSQL)
    • First admin user

  Press Enter to begin.
`).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true)

	welcomeFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header("Welcome"), 1, 0, false).
		AddItem(welcomeText, 0, 1, false).
		AddItem(tview.NewButton("[ Begin Setup ]").SetSelectedFunc(func() {
			pages.SwitchToPage("page1")
		}), 3, 0, true).
		AddItem(hint("  Enter to continue  ·  Ctrl+C to quit"), 1, 0, false)

	pages.AddPage("welcome", welcomeFlex, true, true)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			name, _ := pages.GetFrontPage()
			switch name {
			case "page2":
				pages.SwitchToPage("page1")
			case "page3":
				pages.SwitchToPage("page2")
			case "page3a":
				pages.SwitchToPage("page3")
			case "page4":
				pages.SwitchToPage("page3")
			}
			return nil
		}
		return event
	})

	return app.Run()
}
