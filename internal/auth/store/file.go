package store

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

type fileEntry struct {
	password   string
	maxDevices int
}

type FileStore struct {
	path  string
	mu    sync.RWMutex
	users map[string]fileEntry
}

type fileCredentials struct {
	Clients []struct {
		Username   string `toml:"username"`
		Password   string `toml:"password"`
		MaxDevices int    `toml:"max_devices"`
	} `toml:"client"`
}

func NewFileStore(path string) *FileStore {
	s := &FileStore{path: path, users: make(map[string]fileEntry)}
	if err := s.reload(); err != nil {
		slog.Warn("filestore: initial load", "err", err)
	}
	go s.watchLoop()
	return s
}

func (s *FileStore) GetPassword(_ context.Context, username string) (string, error) {
	s.mu.RLock()
	e := s.users[username]
	s.mu.RUnlock()
	return e.password, nil
}

// GetMaxDevices implements auth.DeviceLimitStore.
func (s *FileStore) GetMaxDevices(_ context.Context, username string) (int, error) {
	s.mu.RLock()
	e := s.users[username]
	s.mu.RUnlock()
	return e.maxDevices, nil
}

func (s *FileStore) reload() error {
	var cf fileCredentials
	if _, err := toml.DecodeFile(s.path, &cf); err != nil {
		return err
	}
	m := make(map[string]fileEntry, len(cf.Clients))
	for _, c := range cf.Clients {
		m[c.Username] = fileEntry{password: c.Password, maxDevices: c.MaxDevices}
	}
	s.mu.Lock()
	s.users = m
	s.mu.Unlock()
	slog.Debug("filestore: reloaded", "users", len(m))
	return nil
}

func (s *FileStore) watchLoop() {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for range t.C {
		if err := s.reload(); err != nil {
			slog.Warn("filestore: reload", "err", err)
		}
	}
}
