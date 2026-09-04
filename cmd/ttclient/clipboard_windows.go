//go:build windows

package main

import (
	"os/exec"
	"strings"
)

func readSystemClipboard() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
