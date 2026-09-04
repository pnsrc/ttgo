//go:build !darwin && !windows

package main

import (
	"os/exec"
	"strings"
)

func readSystemClipboard() string {
	out, err := exec.Command("xclip", "-selection", "clipboard", "-o").Output()
	if err != nil {
		out, err = exec.Command("xsel", "--clipboard", "--output").Output()
		if err != nil {
			return ""
		}
	}
	return strings.TrimSpace(string(out))
}
