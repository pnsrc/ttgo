//go:build darwin

package main

import (
	"os/exec"
	"strings"
)

func readSystemClipboard() string {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
