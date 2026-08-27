package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type GlobalSettings struct {
	LastProfileID    string   `json:"last_profile_id"`
	BypassDomains    bool     `json:"bypass_domains"`
	GlobalExclusions []string `json:"global_exclusions"`
	Language         string   `json:"language"`
	AutoConnect      bool     `json:"auto_connect"`
	EnableAdBlock    bool     `json:"enable_adblock"`
	Theme            string   `json:"theme"`
	UpstreamDNS      string   `json:"upstream_dns"`
	RoutingMode      string   `json:"routing_mode"`
}

type SettingsStore struct {
	path string
	mu   sync.Mutex
}

func NewSettingsStore() (*SettingsStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(base, "ttclient")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &SettingsStore{
		path: filepath.Join(dir, "settings.json"),
	}, nil
}

func (s *SettingsStore) Load() (GlobalSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var settings GlobalSettings
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return GlobalSettings{
				BypassDomains: false,
				Language:      "en",
				AutoConnect:   true,
			}, nil
		}
		return settings, err
	}
	err = json.Unmarshal(data, &settings)
	if settings.Language == "" {
		settings.Language = "en"
	}
	return settings, err
}

func (s *SettingsStore) Save(settings GlobalSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
