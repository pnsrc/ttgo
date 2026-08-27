//go:build darwin

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func ensureElevated() {
	if os.Geteuid() == 0 {
		return
	}
	log.Println("Need root privileges. Elevating...")
	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	script := fmt.Sprintf(`do shell script "%s" with administrator privileges`, exe)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to prompt for admin privileges: %v", err)
	}
	os.Exit(0)
}
