package client

import (
	"bufio"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const adBlockURL = "https://raw.githubusercontent.com/Zalexanninev15/NoADS_RU/main/hosts/blocker.txt"

type AdBlocker struct {
	mu      sync.RWMutex
	domains map[string]struct{}
	ready   bool
}

func NewAdBlocker() *AdBlocker {
	a := &AdBlocker{
		domains: make(map[string]struct{}),
	}
	go a.load()
	return a
}

func (a *AdBlocker) load() {
	// Попробуем загрузить из кеша сначала
	cachePath := filepath.Join(os.TempDir(), "ttgo_adblock_cache.txt")
	info, err := os.Stat(cachePath)
	useCache := false
	if err == nil && time.Since(info.ModTime()) < 24*time.Hour {
		useCache = true
	}

	var reader io.Reader
	var f *os.File

	if useCache {
		f, err = os.Open(cachePath)
		if err == nil {
			reader = f
			slog.Info("adblock loading from cache", "path", cachePath)
		} else {
			useCache = false
		}
	}

	if !useCache {
		slog.Info("adblock downloading list", "url", adBlockURL)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(adBlockURL)
		if err != nil {
			slog.Warn("adblock failed to download", "err", err)
			return
		}
		defer resp.Body.Close()

		// Пишем в кеш
		f, err = os.Create(cachePath)
		if err == nil {
			io.Copy(f, resp.Body)
			f.Close()
			f, _ = os.Open(cachePath)
			reader = f
			slog.Info("adblock cached to", "path", cachePath)
		} else {
			reader = resp.Body
		}
	}

	if f != nil {
		defer f.Close()
	}
	if reader == nil {
		return
	}

	start := time.Now()
	domains := make(map[string]struct{}, 200000)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == "0.0.0.0" {
			domains[parts[1]] = struct{}{}
		}
	}

	a.mu.Lock()
	a.domains = domains
	a.ready = true
	a.mu.Unlock()

	slog.Info("adblock loaded", "count", len(domains), "took", time.Since(start))
}

func (a *AdBlocker) IsBlocked(domain string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.ready {
		return false
	}
	domain = strings.TrimSuffix(domain, ".")
	_, ok := a.domains[domain]
	return ok
}
