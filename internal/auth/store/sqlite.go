package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SQLiteStore хранит пользователей в SQLite.
//
// Схема:
//
//	CREATE TABLE IF NOT EXISTS users (
//	    username TEXT PRIMARY KEY,
//	    password TEXT NOT NULL
//	);
type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("sqlite ping: %w", err)
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
	// Миграция существующих БД без max_devices
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN max_devices INTEGER NOT NULL DEFAULT 0`)
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) GetPassword(ctx context.Context, username string) (string, error) {
	var pw string
	err := s.db.QueryRowContext(ctx,
		`SELECT password FROM users WHERE username = ?`, username,
	).Scan(&pw)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return pw, err
}

// GetMaxDevices implements auth.DeviceLimitStore.
func (s *SQLiteStore) GetMaxDevices(ctx context.Context, username string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT max_devices FROM users WHERE username = ?`, username,
	).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
