package admin

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RunManage runs the management TUI.
func RunManage(paths Paths) error {
	// Открываем store согласно vpn.toml (file / sqlite / postgres)
	store, err := OpenStoreFromVPN(paths)
	if err != nil {
		return fmt.Errorf("open user store: %w", err)
	}

	app := tview.NewApplication()
	pages := tview.NewPages()

	// ── bars ─────────────────────────────────────────────────────────────────

	statusBar := tview.NewTextView().SetDynamicColors(true).SetText(" Ready")
	hintsBar := tview.NewTextView().SetDynamicColors(true).
		SetText("  [yellow]1[white] Users  [yellow]2[white] Certs  [yellow]3[white] Status  [yellow]Ctrl+C[white] Quit")

	storeLabel := fmt.Sprintf("[gray]store: %s (%s)[white]", store.StoreType(), store.Location())
	topBar := tview.NewTextView().SetDynamicColors(true).
		SetText(fmt.Sprintf(" [aqua::b]TrustTunnel Admin[white]   %s", storeLabel))
	topBar.SetBackgroundColor(tcell.ColorDarkSlateGray)

	setStatus := func(msg string, isErr bool) {
		color := "green"
		if isErr {
			color = "red"
		}
		statusBar.SetText(fmt.Sprintf(" [%s]%s[white]", color, msg))
	}

	setStatusAsync := func(msg string, isErr bool) {
		app.QueueUpdateDraw(func() { setStatus(msg, isErr) })
	}

	// ── modal helpers ─────────────────────────────────────────────────────────

	showModal := func(msg string) {
		modal := tview.NewModal().
			SetText(msg).
			AddButtons([]string{"OK"}).
			SetDoneFunc(func(_ int, _ string) { pages.RemovePage("modal") })
		pages.AddPage("modal", modal, true, true)
	}

	confirm := func(msg string, yes func()) {
		modal := tview.NewModal().
			SetText(msg).
			AddButtons([]string{"Yes", "No"}).
			SetDoneFunc(func(_ int, label string) {
				pages.RemovePage("confirm")
				if label == "Yes" {
					yes()
				}
			})
		pages.AddPage("confirm", modal, true, true)
	}

	// ══════════════════════════════════════════════════════════════════════════
	// PAGE: USERS
	// ══════════════════════════════════════════════════════════════════════════

	usersTable := tview.NewTable().SetBorders(false).SetSelectable(true, false).SetFixed(1, 0)
	usersTable.SetBorder(true).
		SetTitle(fmt.Sprintf(" Users [%s]  [yellow](A)[white]dd  [yellow](D)[white]elete  [yellow](P)[white]assword  [yellow](R)[white]efresh ",
			store.StoreType()))

	refreshUsers := func() {
		usersTable.Clear()
		usersTable.SetCell(0, 0, tview.NewTableCell("[::b]USERNAME").
			SetTextColor(tcell.ColorYellow).SetSelectable(false).SetExpansion(2))
		usersTable.SetCell(0, 1, tview.NewTableCell("[::b]STORE").
			SetTextColor(tcell.ColorYellow).SetSelectable(false))

		entries, err := store.List()
		if err != nil {
			setStatus("Error: "+err.Error(), true)
			return
		}
		for i, e := range entries {
			usersTable.SetCell(i+1, 0, tview.NewTableCell(e.Username).SetExpansion(2))
			usersTable.SetCell(i+1, 1, tview.NewTableCell(store.StoreType()).
				SetTextColor(tcell.ColorGray))
		}
		if len(entries) == 0 {
			usersTable.SetCell(1, 0, tview.NewTableCell("(no users)").
				SetTextColor(tcell.ColorGray).SetSelectable(false))
		}
	}

	buildAddForm := func() {
		form := tview.NewForm().
			AddInputField("Username", "", 30, nil, nil).
			AddPasswordField("Password", "", 30, '*', nil)
		form.SetBorder(true).SetTitle(" Add User ")
		form.AddButton("Add", func() {
			u := strings.TrimSpace(form.GetFormItem(0).(*tview.InputField).GetText())
			p := form.GetFormItem(1).(*tview.InputField).GetText()
			if u == "" || p == "" {
				showModal("Username and password cannot be empty")
				return
			}
			if err := store.Add(u, p); err != nil {
				showModal("Error: " + err.Error())
				return
			}
			pages.RemovePage("adduser")
			refreshUsers()
			setStatus(fmt.Sprintf("User %q added", u), false)
			app.SetFocus(usersTable)
		})
		form.AddButton("Cancel", func() {
			pages.RemovePage("adduser")
			app.SetFocus(usersTable)
		})
		pages.AddPage("adduser", centered(form, 50, 12), true, true)
		app.SetFocus(form)
	}

	buildChangePassForm := func(username string) {
		form := tview.NewForm().
			AddPasswordField("New password", "", 30, '*', nil).
			AddPasswordField("Confirm", "", 30, '*', nil)
		form.SetBorder(true).SetTitle(fmt.Sprintf(" Change password: %s ", username))
		form.AddButton("Save", func() {
			p1 := form.GetFormItem(0).(*tview.InputField).GetText()
			p2 := form.GetFormItem(1).(*tview.InputField).GetText()
			if p1 != p2 {
				showModal("Passwords do not match")
				return
			}
			if p1 == "" {
				showModal("Password cannot be empty")
				return
			}
			if err := store.ChangePassword(username, p1); err != nil {
				showModal("Error: " + err.Error())
				return
			}
			pages.RemovePage("chpass")
			setStatus(fmt.Sprintf("Password changed for %q", username), false)
			app.SetFocus(usersTable)
		})
		form.AddButton("Cancel", func() {
			pages.RemovePage("chpass")
			app.SetFocus(usersTable)
		})
		pages.AddPage("chpass", centered(form, 50, 12), true, true)
		app.SetFocus(form)
	}

	usersTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := usersTable.GetSelection()
		switch event.Rune() {
		case 'a', 'A':
			buildAddForm()
			return nil
		case 'd', 'D':
			if row < 1 {
				return event
			}
			username := usersTable.GetCell(row, 0).Text
			if username == "" || username == "(no users)" {
				return event
			}
			confirm(fmt.Sprintf("Delete user %q?\nExisting connections will be terminated.", username), func() {
				if err := store.Delete(username); err != nil {
					showModal("Error: " + err.Error())
					return
				}
				refreshUsers()
				setStatus(fmt.Sprintf("User %q deleted", username), false)
			})
			return nil
		case 'p', 'P':
			if row < 1 {
				return event
			}
			username := usersTable.GetCell(row, 0).Text
			if username == "" || username == "(no users)" {
				return event
			}
			buildChangePassForm(username)
			return nil
		case 'r', 'R':
			refreshUsers()
			setStatus("Refreshed", false)
			return nil
		}
		return event
	})

	usersPage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(usersTable, 0, 1, true)

	// ══════════════════════════════════════════════════════════════════════════
	// PAGE: CERTIFICATES
	// ══════════════════════════════════════════════════════════════════════════

	certsText := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)
	certsText.SetBorder(true).SetTitle(" Certificates  [yellow](A)[white]dd/Renew  [yellow](R)[white]eload TLS ")

	refreshCerts := func() {
		hf, err := loadHosts(paths.Hosts)
		if err != nil {
			certsText.SetText("[red]Error loading hosts.toml: " + err.Error())
			return
		}

		var sb strings.Builder
		sb.WriteString("[::b]MAIN HOSTS[white]\n\n")
		if len(hf.MainHosts) == 0 {
			sb.WriteString("  [gray](none)[white]\n")
		}
		for _, h := range hf.MainHosts {
			sb.WriteString(fmt.Sprintf("  [yellow]%s[white]\n", h.Hostname))
			sb.WriteString(fmt.Sprintf("    cert: %s\n", h.CertChainPath))
			sb.WriteString(fmt.Sprintf("    key:  %s\n", h.PrivateKeyPath))
			if domains, expiry, err := CertInfo(h.CertChainPath); err == nil {
				days := int(time.Until(expiry).Hours() / 24)
				col := "green"
				if days < 30 {
					col = "red"
				} else if days < 60 {
					col = "yellow"
				}
				if len(domains) > 0 {
					sb.WriteString(fmt.Sprintf("    domains: %s\n", strings.Join(domains, ", ")))
				}
				sb.WriteString(fmt.Sprintf("    expires: [%s]%s (%d days)[white]\n",
					col, expiry.Format("2006-01-02"), days))
			} else {
				sb.WriteString(fmt.Sprintf("    [red]cert info unavailable: %s[white]\n", err))
			}
			sb.WriteString("\n")
		}
		certsText.SetText(sb.String())
	}

	buildAddCertForm := func() {
		form := tview.NewForm().
			AddInputField("Hostname / IP", "", 40, nil, nil).
			AddDropDown("TLS type", []string{
				"Let's Encrypt",
				"Self-signed",
				"Custom paths",
			}, 0, nil).
			AddInputField("Email (Let's Encrypt only)", "", 40, nil, nil).
			AddInputField("Cert path (custom only)", "", 50, nil, nil).
			AddInputField("Key path (custom only)", "", 50, nil, nil)
		form.SetBorder(true).SetTitle(" Add / Renew Certificate ")

		form.AddButton("Apply", func() {
			hostname := strings.TrimSpace(form.GetFormItem(0).(*tview.InputField).GetText())
			tlsIdx, _ := form.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
			email := strings.TrimSpace(form.GetFormItem(2).(*tview.InputField).GetText())
			customCert := strings.TrimSpace(form.GetFormItem(3).(*tview.InputField).GetText())
			customKey := strings.TrimSpace(form.GetFormItem(4).(*tview.InputField).GetText())

			if hostname == "" {
				showModal("Hostname is required")
				return
			}
			pages.RemovePage("addcert")
			app.SetFocus(certsText)

			var cPath, kPath string
			switch tlsIdx {
			case 0:
				if email == "" {
					showModal("Email is required for Let's Encrypt")
					return
				}
				cPath = "/etc/trusttunnel/certs/" + hostname + ".crt"
				kPath = "/etc/trusttunnel/certs/" + hostname + ".key"
				setStatus("Obtaining Let's Encrypt certificate...", false)
				go func() {
					err := ObtainCert(hostname, email, cPath, kPath, func(s string) {
						setStatusAsync(s, false)
					})
					app.QueueUpdateDraw(func() {
						if err != nil {
							showModal("Let's Encrypt error: " + err.Error())
							return
						}
						addOrUpdateHost(paths, hostname, cPath, kPath)
						refreshCerts()
						reloadTLS()
						setStatus("Certificate obtained, TLS reloaded", false)
					})
				}()
				return
			case 1:
				cPath = "/etc/trusttunnel/certs/" + hostname + ".crt"
				kPath = "/etc/trusttunnel/certs/" + hostname + ".key"
				if err := GenerateSelfSigned(hostname, cPath, kPath); err != nil {
					showModal("Error: " + err.Error())
					return
				}
			case 2:
				cPath, kPath = customCert, customKey
				if err := VerifyCert(cPath, kPath); err != nil {
					showModal("Cert verify failed: " + err.Error())
					return
				}
			}

			addOrUpdateHost(paths, hostname, cPath, kPath)
			refreshCerts()
			reloadTLS()
			setStatus(fmt.Sprintf("Cert for %s added, TLS reloaded", hostname), false)
		})
		form.AddButton("Cancel", func() {
			pages.RemovePage("addcert")
			app.SetFocus(certsText)
		})
		pages.AddPage("addcert", centered(form, 65, 20), true, true)
		app.SetFocus(form)
	}

	certsText.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'a', 'A':
			buildAddCertForm()
			return nil
		case 'r', 'R':
			if err := reloadTLS(); err != nil {
				setStatus("Reload failed: "+err.Error(), true)
			} else {
				setStatus("SIGHUP sent — TLS reloaded", false)
			}
		}
		return event
	})

	certsPage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(certsText, 0, 1, true)

	// ══════════════════════════════════════════════════════════════════════════
	// PAGE: STATUS
	// ══════════════════════════════════════════════════════════════════════════

	statusText := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)
	statusText.SetBorder(true).SetTitle(" Server Status  [yellow](R)[white]eload TLS  [yellow](F5)[white] Refresh ")

	refreshStatus := func() {
		var sb strings.Builder
		if serverRunning() {
			pid, _ := serverPID()
			sb.WriteString(fmt.Sprintf("[green]● RUNNING[white]  pid=%d\n\n", pid))
		} else {
			sb.WriteString("[red]● NOT RUNNING[white]\n\n")
		}

		sb.WriteString("[::b]Config files:[white]\n")
		for _, f := range []struct{ name, path string }{
			{"vpn.toml", paths.VPN},
			{"hosts.toml", paths.Hosts},
			{"credentials.toml (file store)", paths.Creds},
		} {
			icon := "[green]✓[white]"
			if !fileExistsFn(f.path) {
				icon = "[gray]–[white]"
			}
			sb.WriteString(fmt.Sprintf("  %s %s\n     → %s\n", icon, f.name, f.path))
		}

		sb.WriteString(fmt.Sprintf("\n[::b]User store:[white] [yellow]%s[white] (%s)\n",
			store.StoreType(), store.Location()))

		if users, err := store.List(); err == nil {
			sb.WriteString(fmt.Sprintf("  %d user(s) in store\n", len(users)))
		}

		statusText.SetText(sb.String())
	}

	statusText.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyF5:
			refreshStatus()
			setStatus("Refreshed", false)
		case event.Rune() == 'r' || event.Rune() == 'R':
			if err := reloadTLS(); err != nil {
				setStatus("Reload failed: "+err.Error(), true)
			} else {
				setStatus("SIGHUP sent — TLS reloaded", false)
			}
		}
		return event
	})

	statusPage := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(statusText, 0, 1, true)

	// ── assemble ──────────────────────────────────────────────────────────────

	pages.AddPage("users", usersPage, true, true)
	pages.AddPage("certs", certsPage, true, false)
	pages.AddPage("status", statusPage, true, false)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(topBar, 1, 0, false).
		AddItem(pages, 0, 1, true).
		AddItem(statusBar, 1, 0, false).
		AddItem(hintsBar, 1, 0, false)

	app.SetRoot(root, true).SetFocus(usersTable)

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		front, _ := pages.GetFrontPage()
		if front != "users" && front != "certs" && front != "status" {
			return event
		}
		switch event.Rune() {
		case '1':
			pages.SwitchToPage("users")
			refreshUsers()
			app.SetFocus(usersTable)
		case '2':
			pages.SwitchToPage("certs")
			refreshCerts()
			app.SetFocus(certsText)
		case '3':
			pages.SwitchToPage("status")
			refreshStatus()
			app.SetFocus(statusText)
		}
		return event
	})

	refreshUsers()
	return app.Run()
}

// ── helpers ───────────────────────────────────────────────────────────────────

func centered(p tview.Primitive, w, h int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, h, 0, true).
			AddItem(nil, 0, 1, false), w, 0, true).
		AddItem(nil, 0, 1, false)
}

func addOrUpdateHost(paths Paths, hostname, certPath, keyPath string) {
	hf, err := loadHosts(paths.Hosts)
	if err != nil {
		hf = &hostsFile{}
	}
	entry := hostEntry{Hostname: hostname, CertChainPath: certPath, PrivateKeyPath: keyPath}
	updated := false
	for i, h := range hf.MainHosts {
		if h.Hostname == hostname {
			hf.MainHosts[i] = entry
			updated = true
			break
		}
	}
	if !updated {
		hf.MainHosts = append(hf.MainHosts, entry)
	}
	updatedPing := false
	for i, h := range hf.PingHosts {
		if h.Hostname == hostname {
			hf.PingHosts[i] = entry
			updatedPing = true
			break
		}
	}
	if !updatedPing {
		hf.PingHosts = append(hf.PingHosts, entry)
	}
	saveHosts(paths.Hosts, hf)
}
