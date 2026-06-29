package admin

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// --- credentials.toml ---

type credFile struct {
	Clients []credEntry `toml:"client"`
}

type credEntry struct {
	Username    string `toml:"username"`
	Password    string `toml:"password"`
	MaxDevices  int    `toml:"max_devices,omitempty"`
	// Lifecycle поля (только sqlite/postgres; file store эти поля игнорирует)
	Enabled         bool   `toml:"-"`
	ExpiresAt       int64  `toml:"-"`
	TrafficLimit    uint64 `toml:"-"`
	TrafficUsed     uint64 `toml:"-"`
}

func loadCreds(path string) ([]credEntry, error) {
	var cf credFile
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	if _, err := toml.DecodeFile(path, &cf); err != nil {
		return nil, err
	}
	return cf.Clients, nil
}

func saveCreds(path string, entries []credEntry) error {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString("[[client]]\n")
		sb.WriteString(fmt.Sprintf("username = %q\n", e.Username))
		sb.WriteString(fmt.Sprintf("password = %q\n", e.Password))
		if e.MaxDevices > 0 {
			sb.WriteString(fmt.Sprintf("max_devices = %d\n", e.MaxDevices))
		}
		sb.WriteString("\n")
	}
	return os.WriteFile(path, []byte(sb.String()), 0600)
}

func addUser(path, username, password string) error {
	entries, err := loadCreds(path)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Username == username {
			return fmt.Errorf("user %q already exists", username)
		}
	}
	entries = append(entries, credEntry{Username: username, Password: password})
	return saveCreds(path, entries)
}

func deleteUser(path, username string) error {
	entries, err := loadCreds(path)
	if err != nil {
		return err
	}
	filtered := entries[:0]
	for _, e := range entries {
		if e.Username != username {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == len(entries) {
		return fmt.Errorf("user %q not found", username)
	}
	return saveCreds(path, filtered)
}

func changePassword(path, username, newPassword string) error {
	entries, err := loadCreds(path)
	if err != nil {
		return err
	}
	for i, e := range entries {
		if e.Username == username {
			entries[i].Password = newPassword
			return saveCreds(path, entries)
		}
	}
	return fmt.Errorf("user %q not found", username)
}

// --- hosts.toml ---

type hostsFile struct {
	MainHosts      []hostEntry `toml:"main_hosts"`
	PingHosts      []hostEntry `toml:"ping_hosts"`
	SpeedtestHosts []hostEntry `toml:"speedtest_hosts"`
}

type hostEntry struct {
	Hostname      string `toml:"hostname"`
	CertChainPath string `toml:"cert_chain_path"`
	PrivateKeyPath string `toml:"private_key_path"`
}

func loadHosts(path string) (*hostsFile, error) {
	var hf hostsFile
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &hf, nil
	}
	if _, err := toml.DecodeFile(path, &hf); err != nil {
		return nil, err
	}
	return &hf, nil
}

func saveHosts(path string, hf *hostsFile) error {
	var sb strings.Builder
	writeHosts := func(section string, entries []hostEntry) {
		for _, e := range entries {
			sb.WriteString(fmt.Sprintf("[[%s]]\n", section))
			sb.WriteString(fmt.Sprintf("hostname = %q\n", e.Hostname))
			sb.WriteString(fmt.Sprintf("cert_chain_path = %q\n", e.CertChainPath))
			sb.WriteString(fmt.Sprintf("private_key_path = %q\n\n", e.PrivateKeyPath))
		}
	}
	writeHosts("main_hosts", hf.MainHosts)
	writeHosts("ping_hosts", hf.PingHosts)
	writeHosts("speedtest_hosts", hf.SpeedtestHosts)
	return os.WriteFile(path, []byte(sb.String()), 0644)
}
