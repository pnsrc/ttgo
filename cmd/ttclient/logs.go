package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func logFilePath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "ttclient", "ttclient.log")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ttclient", "ttclient.log")
}

func readLogTail(maxLines int) (string, error) {
	data, err := os.ReadFile(logFilePath())
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n"), nil
}
