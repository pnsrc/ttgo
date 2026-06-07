package admin

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

// UserStore — единый интерфейс для любого хранилища пользователей.
type UserStore interface {
	List() ([]credEntry, error)
	Add(username, password string) error
	Delete(username string) error
	ChangePassword(username, password string) error
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
			username TEXT PRIMARY KEY,
			password TEXT NOT NULL
		)
	`); err != nil {
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}
	return &sqliteUserStore{dsn: dsn, db: db}, nil
}

func (s *sqliteUserStore) StoreType() string  { return "sqlite" }
func (s *sqliteUserStore) Location() string   { return s.dsn }

func (s *sqliteUserStore) List() ([]credEntry, error) {
	rows, err := s.db.Query(`SELECT username, password FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []credEntry
	for rows.Next() {
		var e credEntry
		if err := rows.Scan(&e.Username, &e.Password); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
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
