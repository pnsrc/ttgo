//go:build windows

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func ensureElevated() {
	if isAdmin() {
		return
	}
	log.Println("Need administrator privileges. Elevating via UAC...")
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"Start-Process", "'"+exe+"'", "-Verb", "RunAs",
		"-ArgumentList", "'"+strings.Join(os.Args[1:], " ")+"'")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to elevate: %v", err)
	}
	os.Exit(0)
}

func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		return false
	}
	return true
}

func deepLinkFile() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "ttclient", "pending_deeplink")
}

func writeDeepLinkFile(url string) {
	dir := filepath.Dir(deepLinkFile())
	os.MkdirAll(dir, 0755)
	os.WriteFile(deepLinkFile(), []byte(url), 0644)
}

func readAndClearDeepLinkFile() string {
	path := deepLinkFile()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	os.Remove(path)
	return strings.TrimSpace(string(data))
}

func init() {
	// Hide console window for GUI app on Windows
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	user32 := syscall.NewLazyDLL("user32.dll")
	showWindow := user32.NewProc("ShowWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd != 0 {
		showWindow.Call(hwnd, 0) // SW_HIDE
	}

	registerDeepLinkProtocol()
}

func registerDeepLinkProtocol() {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	commands := []string{
		`New-Item -Path 'HKCU:\Software\Classes\firetunnel' -Force | Out-Null`,
		`Set-ItemProperty -Path 'HKCU:\Software\Classes\firetunnel' -Name '(Default)' -Value 'URL:FireTunnel Protocol'`,
		`Set-ItemProperty -Path 'HKCU:\Software\Classes\firetunnel' -Name 'URL Protocol' -Value ''`,
		`New-Item -Path 'HKCU:\Software\Classes\firetunnel\shell\open\command' -Force | Out-Null`,
		`Set-ItemProperty -Path 'HKCU:\Software\Classes\firetunnel\shell\open\command' -Name '(Default)' -Value '"` + exe + `" "%1"'`,
	}

	for _, c := range commands {
		cmd := exec.Command("powershell", "-NoProfile", "-Command", c)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Run()
	}
}
