package client

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ─── on-disk format ──────────────────────────────────────────────────────────
//
// Стандартный TrustTunnel client config. Парсим все поля, но реально
// используем только endpoint + listener.tun. Остальные сохраняем для
// round-trip при экспорте.

type ProfileTOML struct {
	LogLevel               string   `toml:"loglevel,omitempty"`
	VPNMode                string   `toml:"vpn_mode,omitempty"`
	KillswitchEnabled      bool     `toml:"killswitch_enabled,omitempty"`
	PostQuantumEnabled     bool     `toml:"post_quantum_group_enabled,omitempty"`
	Exclusions             []string `toml:"exclusions,omitempty"`
	DNSUpstreams           []string `toml:"dns_upstreams,omitempty"`

	Endpoint EndpointTOML `toml:"endpoint"`
	Listener ListenerTOML `toml:"listener,omitempty"`
}

type ListenerTOML struct {
	TUN   *TunTOML   `toml:"tun,omitempty"`
	SOCKS *SocksTOML `toml:"socks,omitempty"`
}

type EndpointTOML struct {
	Hostname                 string   `toml:"hostname"`
	Addresses                []string `toml:"addresses"`
	Username                 string   `toml:"username"`
	Password                 string   `toml:"password"`
	ClientRandom             string   `toml:"client_random,omitempty"`
	CustomSNI                string   `toml:"custom_sni,omitempty"`
	HasIPv6                  bool     `toml:"has_ipv6,omitempty"`
	SkipVerification         bool     `toml:"skip_verification,omitempty"`
	UpstreamProtocol         string   `toml:"upstream_protocol,omitempty"`
	UpstreamFallbackProtocol string   `toml:"upstream_fallback_protocol,omitempty"`
	AntiDPI                  bool     `toml:"anti_dpi,omitempty"`
	Certificate              string   `toml:"certificate,omitempty"`
}

type TunTOML struct {
	BoundIf         string   `toml:"bound_if,omitempty"`
	MTU             int      `toml:"mtu_size,omitempty"`
	ChangeSystemDNS bool     `toml:"change_system_dns,omitempty"`
	IncludedRoutes  []string `toml:"included_routes,omitempty"`
	ExcludedRoutes  []string `toml:"excluded_routes,omitempty"`
}

type SocksTOML struct {
	Address  string `toml:"address,omitempty"`
	Username string `toml:"username,omitempty"`
	Password string `toml:"password,omitempty"`
}

// Profile — то что в UI: ID + display name + сам конфиг.
type Profile struct {
	ID       string      `json:"id"`        // sha1[:8] от пути файла
	Path     string      `json:"path"`      // путь к .toml на диске
	Name     string      `json:"name"`      // отображаемое имя (basename без .toml)
	Endpoint string      `json:"endpoint"`  // первый адрес из endpoint.addresses (для preview)
	Username string      `json:"username"`
	Toml     ProfileTOML `json:"toml"`
}

// ToConfig преобразует профиль в Config для Dialer.
// addresses[0] выбирается как endpoint host:port; hostname берётся из
// endpoint.hostname если задан, иначе из самого address.
func (p *Profile) ToConfig() Config {
	addr := ""
	if len(p.Toml.Endpoint.Addresses) > 0 {
		addr = p.Toml.Endpoint.Addresses[0]
	}
	// addresses часто содержат hostname:port — оставляем как есть для
	// возможности резолва, но устанавливаем SNI отдельно.
	sni := p.Toml.Endpoint.CustomSNI
	if sni == "" {
		sni = p.Toml.Endpoint.Hostname
	}
	return Config{
		Endpoint:      addr,
		Hostname:      sni,
		Username:      p.Toml.Endpoint.Username,
		Password:      p.Toml.Endpoint.Password,
		Insecure:      p.Toml.Endpoint.SkipVerification,
		PinnedCertPEM: []byte(p.Toml.Endpoint.Certificate),
	}
}

// ─── ProfileStore ────────────────────────────────────────────────────────────

// ProfileStore — файловое хранилище профилей.
// macOS:   ~/Library/Application Support/ttclient/profiles
// Linux:   ~/.config/ttclient/profiles
// Windows: %AppData%\ttclient\profiles
type ProfileStore struct {
	dir string
}

func NewProfileStore() (*ProfileStore, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(base, "ttclient", "profiles")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &ProfileStore{dir: dir}, nil
}

// Dir returns the profiles directory path (for UI display).
func (s *ProfileStore) Dir() string { return s.dir }

// List loads all *.toml profiles from disk, sorted by name.
func (s *ProfileStore) List() ([]Profile, error) {
	var out []Profile
	err := filepath.WalkDir(s.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".toml") {
			return nil
		}
		p, err := loadProfile(path)
		if err != nil {
			return nil // skip invalid
		}
		out = append(out, *p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Import copies an external .toml into the store with a sanitized filename.
// If a profile with the same name exists — appends a timestamp.
func (s *ProfileStore) Import(srcPath string) (*Profile, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	return s.ImportContent(data, filepath.Base(srcPath))
}

// ImportContent сохраняет содержимое .toml как новый профиль с именем filename.
// Полезно при drop файла через Wails — данные приходят уже в памяти.
func (s *ProfileStore) ImportContent(data []byte, filename string) (*Profile, error) {
	// Сначала валидируем парсингом
	var doc ProfileTOML
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	if doc.Endpoint.Username == "" || len(doc.Endpoint.Addresses) == 0 {
		return nil, fmt.Errorf("profile must have endpoint.username and endpoint.addresses")
	}

	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = sanitizeFilename(name)
	if name == "" {
		name = "profile"
	}

	target := filepath.Join(s.dir, name+".toml")
	if _, err := os.Stat(target); err == nil {
		target = filepath.Join(s.dir, fmt.Sprintf("%s-%d.toml", name, time.Now().Unix()))
	}

	if err := os.WriteFile(target, data, 0600); err != nil {
		return nil, err
	}
	return loadProfile(target)
}

// Delete removes a profile by ID.
func (s *ProfileStore) Delete(id string) error {
	list, err := s.List()
	if err != nil {
		return err
	}
	for _, p := range list {
		if p.ID == id {
			return os.Remove(p.Path)
		}
	}
	return fmt.Errorf("profile %s not found", id)
}

// Get returns a profile by ID.
func (s *ProfileStore) Get(id string) (*Profile, error) {
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("profile %s not found", id)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func loadProfile(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc ProfileTOML
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	endpoint := ""
	if len(doc.Endpoint.Addresses) > 0 {
		endpoint = doc.Endpoint.Addresses[0]
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	h := sha1.Sum([]byte(path))
	return &Profile{
		ID:       hex.EncodeToString(h[:])[:8],
		Path:     path,
		Name:     name,
		Endpoint: endpoint,
		Username: doc.Endpoint.Username,
		Toml:     doc,
	}, nil
}

// sanitizeFilename убирает опасные символы из имени файла.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	return b.String()
}
