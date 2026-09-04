//go:build darwin

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	registerURLScheme()
}

func registerURLScheme() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	appPath := exe
	for appPath != "/" && appPath != "." {
		if strings.HasSuffix(appPath, ".app") {
			break
		}
		appPath = filepath.Dir(appPath)
	}
	if !strings.HasSuffix(appPath, ".app") {
		return
	}
	lsregister := "/System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/LaunchServices.framework/Versions/A/Support/lsregister"
	exec.Command(lsregister, "-R", "-f", appPath).Run()
	log.Println("Registered URL scheme via lsregister:", appPath)
}

func ensureElevated() {
	if os.Geteuid() == 0 {
		return
	}
	log.Println("Need root privileges. Elevating...")
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	args := strings.Join(os.Args[1:], " ")
	script := fmt.Sprintf(`do shell script "%s %s" with administrator privileges`, exe, args)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to prompt for admin privileges: %v", err)
	}
	os.Exit(0)
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
