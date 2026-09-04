//go:build !darwin && !windows

package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func ensureElevated() {
	if os.Geteuid() == 0 {
		return
	}
	log.Fatalln("Elevated privileges required. Please run with sudo.")
}

func deepLinkFile() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".config", "ttclient", "pending_deeplink")
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
