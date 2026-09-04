//go:build windows

package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ConflictAdapter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func (a *App) CheckWintun() bool {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	_, err := os.Stat(filepath.Join(dir, "wintun.dll"))
	return err == nil
}

func (a *App) DownloadWintun() error {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	dest := filepath.Join(dir, "wintun.dll")

	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	url := "https://www.wintun.net/builds/wintun-0.14.1.zip"
	slog.Info("downloading wintun", "url", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmpZip := filepath.Join(dir, "wintun.zip")
	f, err := os.Create(tmpZip)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpZip)
		return fmt.Errorf("download write: %w", err)
	}
	f.Close()
	defer os.Remove(tmpZip)

	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Expand-Archive -Path '%s' -DestinationPath '%s' -Force`, tmpZip, filepath.Join(dir, "wintun_tmp"))).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unzip: %s: %w", string(out), err)
	}
	defer os.RemoveAll(filepath.Join(dir, "wintun_tmp"))

	dllSrc := filepath.Join(dir, "wintun_tmp", "wintun", "bin", "amd64", "wintun.dll")
	if _, err := os.Stat(dllSrc); err != nil {
		dllSrc = filepath.Join(dir, "wintun_tmp", "wintun", "bin", "arm64", "wintun.dll")
	}

	data, err := os.ReadFile(dllSrc)
	if err != nil {
		return fmt.Errorf("read dll: %w", err)
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return fmt.Errorf("write dll: %w", err)
	}

	slog.Info("wintun.dll installed", "path", dest)
	return nil
}

func (a *App) FindConflictAdapters() []ConflictAdapter {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`Get-NetAdapter | Select-Object Name, InterfaceDescription, Status | ConvertTo-Csv -NoTypeInformation`).Output()
	if err != nil {
		slog.Warn("failed to list adapters", "err", err)
		return nil
	}

	conflicts := []string{
		"radmin", "hamachi", "anydesk", "zerotier", "tailscale",
		"wireguard", "wintun", "cloudflare", "openvpn", "tap-windows",
	}

	var result []ConflictAdapter
	lines := strings.Split(string(out), "\n")
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.Trim(line, "\"")
		parts := strings.Split(line, "\",\"")
		if len(parts) < 3 {
			continue
		}
		name := strings.Trim(parts[0], "\"")
		desc := strings.Trim(parts[1], "\"")
		status := strings.Trim(parts[2], "\"")

		lower := strings.ToLower(name + " " + desc)
		for _, kw := range conflicts {
			if strings.Contains(lower, kw) && !strings.Contains(lower, "firetunnel") {
				result = append(result, ConflictAdapter{
					Name:        name,
					Description: desc,
					Status:      status,
				})
				break
			}
		}
	}
	return result
}

func (a *App) DisableAdapter(name string) error {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Disable-NetAdapter -Name '%s' -Confirm:$false`, name)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("disable adapter %s: %s: %w", name, string(out), err)
	}
	slog.Info("disabled adapter", "name", name)
	return nil
}
