package admin

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RunMigrate runs the interactive migration wizard.
// Supports: file → sqlite.
func RunMigrate(paths Paths) error {
	app := tview.NewApplication()
	pages := tview.NewPages()
	app.SetRoot(pages, true)

	header := func(title string) *tview.TextView {
		tv := tview.NewTextView().
			SetText(" TrustTunnel — Migration: " + title).
			SetTextColor(tcell.ColorAqua)
		tv.SetBackgroundColor(tcell.ColorDarkSlateGray)
		return tv
	}

	showResult := func(lines []string, isErr bool) {
		color := "green"
		if isErr {
			color = "red"
		}
		text := tview.NewTextView().SetDynamicColors(true).SetWordWrap(true)
		var sb strings.Builder
		for _, l := range lines {
			sb.WriteString(fmt.Sprintf("[%s]%s[white]\n", color, l))
		}
		text.SetText(sb.String())
		text.SetBorder(true).SetTitle(" Result ")

		flex := tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(header("Done"), 1, 0, false).
			AddItem(text, 0, 1, false).
			AddItem(tview.NewButton("[ Quit ]").SetSelectedFunc(func() {
				app.Stop()
			}), 3, 0, true)
		pages.AddPage("result", flex, true, true)
		app.SetFocus(flex)
	}

	doMigrate := func(srcCredPath, targetDSN string, patchVPN bool) {
		var log []string
		addLog := func(s string) { log = append(log, s) }

		// 1. Читаем текущих пользователей
		addLog(fmt.Sprintf("Reading users from %s ...", srcCredPath))
		entries, err := loadCreds(srcCredPath)
		if err != nil {
			showResult(append(log, "ERROR: "+err.Error()), true)
			return
		}
		addLog(fmt.Sprintf("Found %d user(s)", len(entries)))

		// 2. Открываем / создаём SQLite
		addLog(fmt.Sprintf("Opening SQLite: %s ...", targetDSN))
		store, err := NewSQLiteStore(targetDSN)
		if err != nil {
			showResult(append(log, "ERROR: "+err.Error()), true)
			return
		}

		// 3. Мигрируем пользователей (skip duplicates)
		migrated, skipped := 0, 0
		for _, e := range entries {
			if err := store.Add(e.Username, e.Password); err != nil {
				addLog(fmt.Sprintf("  SKIP %s: %s", e.Username, err))
				skipped++
			} else {
				addLog(fmt.Sprintf("  OK   %s", e.Username))
				migrated++
			}
		}
		addLog(fmt.Sprintf("Migrated: %d, Skipped (duplicates): %d", migrated, skipped))

		// 4. Обновляем vpn.toml
		if patchVPN {
			addLog(fmt.Sprintf("Patching %s ...", paths.VPN))
			if err := patchVPNStoreType(paths.VPN, "sqlite", targetDSN, ""); err != nil {
				showResult(append(log, "ERROR patching vpn.toml: "+err.Error()), true)
				return
			}
			addLog("  store_type = \"sqlite\"")
			addLog(fmt.Sprintf("  store_dsn  = %q", targetDSN))
		}

		addLog("")
		addLog("Migration complete!")
		if patchVPN {
			addLog("Restart or send SIGHUP to apply new store settings.")
		}
		showResult(log, false)
	}

	// ── form ──────────────────────────────────────────────────────────────────

	// Дефолты из текущего конфига
	defaultDSN := "users.db"
	defaultSrc := paths.Creds
	patchVPN := true

	if cfg, err := loadVPNConfig(paths.VPN); err == nil {
		if cfg.StoreDSN != "" {
			defaultDSN = cfg.StoreDSN
		}
		if cfg.CredentialsFile != "" {
			defaultSrc = cfg.CredentialsFile
		}
	}

	var form *tview.Form
	form = tview.NewForm().
		AddTextView("From", "", 0, 1, false, false).
		AddInputField("Source credentials.toml", defaultSrc, 50, nil, nil).
		AddTextView("", "", 0, 1, false, false).
		AddTextView("To", "", 0, 1, false, false).
		AddInputField("SQLite file path", defaultDSN, 50, nil, nil).
		AddCheckbox("Patch vpn.toml (store_type=sqlite)", patchVPN, func(v bool) { patchVPN = v })
	form.SetBorder(true).SetTitle(" Migrate: file → SQLite ")

	// Preview users
	previewText := tview.NewTextView().SetDynamicColors(true)
	previewText.SetBorder(true).SetTitle(" Users to migrate (preview) ")

	updatePreview := func() {
		src := form.GetFormItem(1).(*tview.InputField).GetText()
		entries, err := loadCreds(src)
		if err != nil || len(entries) == 0 {
			previewText.SetText("[gray](no users found or file not readable)[white]")
			return
		}
		var sb strings.Builder
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("  [yellow]%-20s[white] ****\n", e.Username))
		}
		sb.WriteString(fmt.Sprintf("\n  [gray]Total: %d user(s)[white]", len(entries)))
		previewText.SetText(sb.String())
	}
	updatePreview()

	form.AddButton("Preview", func() {
		updatePreview()
		app.SetFocus(previewText)
	})

	form.AddButton("Migrate →", func() {
		src := strings.TrimSpace(form.GetFormItem(1).(*tview.InputField).GetText())
		dst := strings.TrimSpace(form.GetFormItem(4).(*tview.InputField).GetText())
		if src == "" || dst == "" {
			return
		}
		doMigrate(src, dst, patchVPN)
	})

	form.AddButton("Cancel", func() { app.Stop() })

	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 0, 2, true).
		AddItem(previewText, 0, 1, false)

	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header("file → SQLite"), 1, 0, false).
		AddItem(content, 0, 1, true).
		AddItem(tview.NewTextView().SetText("  Tab/Enter to navigate  ·  Ctrl+C to quit").
			SetTextColor(tcell.ColorGray), 1, 0, false)

	pages.AddPage("main", mainFlex, true, true)
	app.SetFocus(form)

	return app.Run()
}
