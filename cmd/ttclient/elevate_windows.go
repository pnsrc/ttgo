//go:build windows

package main

import (
	"log"
	"os"
	"os/exec"
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
}
