package admin

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

// UserStore — единый интерфейс для любого хранилища пользователей.
type UserStore interface {
	List() ([]credEntry, error)
	Add(username, password string) error
	Delete(username string) error
	ChangePassword(username, password string) error
	SetMaxDevices(username string, n int) error
	StoreType() string
	// DSN / path — для отображения в TUI
	Location() string
}

// ── File store ────────────────────────────────────────────────────────────────

type fileUserStore struct{ path string }

func NewFileStore(path string) UserStore { return &fileUserStore{path} }

func (s *fileUserStore) List() ([]credEntry, error)    { return loadCreds(s.path) }
func (s *fileUserStore) StoreType() string             { return "file" }
func (s *fileUserStore) Location() string              { return s.path }

func (s *fileUserStore) Add(username, password string) error {
	return addUser(s.path, username, password)
}
func (s *fileUserStore) Delete(username string) error {
	return deleteUser(s.path, username)
}
func (s *fileUserStore) ChangePassword(username, password string) error {
	return changePassword(s.path, username, password)
}

func (s *fileUserStore) SetMaxDevices(username string, n int) error {
	entries, err := loadCreds(s.path)
	if err != nil {
		return err
	}
	for i, e := range entries {
		if e.Username == username {
			entries[i].MaxDevices = n
			return saveCreds(s.path, entries)
		}
	}
	return fmt.Errorf("user %q not found", username)
}

// ── SQLite store ──────────────────────────────────────────────────────────────

type sqliteUserStore struct {
	dsn string
	db  *sql.DB
}

func NewSQLiteStore(dsn string) (UserStore, error) {
	if dsn == "" {
		dsn = "users.db"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			username    TEXT PRIMARY KEY,
			password    TEXT NOT NULL,
			max_devices INTEGER NOT NULL DEFAULT 0
		)
	`); err != nil {
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN max_devices INTEGER NOT NULL DEFAULT 0`)
	return &sqliteUserStore{dsn: dsn, db: db}, nil
}

func (s *sqliteUserStore) StoreType() string  { return "sqlite" }
func (s *sqliteUserStore) Location() string   { return s.dsn }

func (s *sqliteUserStore) List() ([]credEntry, error) {
	rows, err := s.db.Query(
		`SELECT username, password, max_devices FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []credEntry
	for rows.Next() {
		var e credEntry
		if err := rows.Scan(&e.Username, &e.Password, &e.MaxDevices); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *sqliteUserStore) SetMaxDevices(username string, n int) error {
	res, err := s.db.Exec(
		`UPDATE users SET max_devices = ? WHERE username = ?`, n, username)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user %q not found", username)
	}
	return nil
}

func (s *sqliteUserStore) Add(username, password string) error {
	_, err := s.db.Exec(
		`INSERT INTO users (username, password) VALUES (?, ?)`, username, password)
	if err != nil {
		return fmt.Errorf("user %q already exists or DB error: %w", username, err)
	}
	return nil
}

func (s *sqliteUserStore) Delete(username string) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE username = ?`, username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %q not found", username)
	}
	return nil
}

func (s *sqliteUserStore) ChangePassword(username, password string) error {
	res, err := s.db.Exec(
		`UPDATE users SET password = ? WHERE username = ?`, password, username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %q not found", username)
	}
	return nil
}

// ── factory ───────────────────────────────────────────────────────────────────

// AdminAPIFromVPN возвращает клиент admin API если он настроен в vpn.toml.
func AdminAPIFromVPN(paths Paths) *AdminClient {
	cfg, err := loadVPNConfig(paths.VPN)
	if err != nil || cfg.Admin == nil || cfg.Admin.Address == "" || cfg.Admin.Token == "" {
		return nil
	}
	addr := cfg.Admin.Address
	// 0.0.0.0 / :: — это listen-адреса, не destination. Переводим в loopback.
	addr = strings.Replace(addr, "0.0.0.0:", "127.0.0.1:", 1)
	addr = strings.Replace(addr, "[::]:", "[::1]:", 1)
	// localhost:9090 → http://localhost:9090
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	return &AdminClient{Address: addr, Token: cfg.Admin.Token}
}

// OpenStoreFromVPN читает vpn.toml и открывает соответствующий UserStore.
func OpenStoreFromVPN(paths Paths) (UserStore, error) {
	cfg, err := loadVPNConfig(paths.VPN)
	if err != nil {
		// Fallback: file store
		return NewFileStore(paths.Creds), nil
	}
	switch cfg.StoreType {
	case "sqlite":
		dsn := cfg.StoreDSN
		if dsn == "" {
			dsn = "users.db"
		}
		return NewSQLiteStore(dsn)
	case "postgres":
		return nil, fmt.Errorf("postgres store is not yet supported in ttadmin (use psql directly)")
	default: // "file" or empty
		credPath := cfg.CredentialsFile
		if credPath == "" {
			credPath = paths.Creds
		}
		return NewFileStore(credPath), nil
	}
}

// fileExists — простая проверка существования файла.
func fileExistsFn(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
