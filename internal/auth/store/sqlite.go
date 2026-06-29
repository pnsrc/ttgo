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
			username             TEXT PRIMARY KEY,
			password             TEXT NOT NULL,
			max_devices          INTEGER NOT NULL DEFAULT 0,
			enabled              INTEGER NOT NULL DEFAULT 1,
			expires_at           INTEGER NOT NULL DEFAULT 0,
			traffic_limit_bytes  INTEGER NOT NULL DEFAULT 0,
			traffic_used_bytes   INTEGER NOT NULL DEFAULT 0
		)
	`); err != nil {
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}
	// Миграция существующих БД
	for _, alt := range []string{
		`ALTER TABLE users ADD COLUMN max_devices INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE users ADD COLUMN expires_at INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN traffic_limit_bytes INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN traffic_used_bytes INTEGER NOT NULL DEFAULT 0`,
	} {
		_, _ = db.Exec(alt)
	}
	return &SQLiteStore{db: db}, nil
}

// GetLifecycle implements auth.LifecycleStore.
func (s *SQLiteStore) GetLifecycle(ctx context.Context, username string) (bool, int64, uint64, uint64, error) {
	var enabled int
	var expiresAt int64
	var limit, used int64
	err := s.db.QueryRowContext(ctx,
		`SELECT enabled, expires_at, traffic_limit_bytes, traffic_used_bytes
		 FROM users WHERE username = ?`, username,
	).Scan(&enabled, &expiresAt, &limit, &used)
	if err == sql.ErrNoRows {
		return false, 0, 0, 0, nil
	}
	if err != nil {
		return false, 0, 0, 0, err
	}
	return enabled != 0, expiresAt, uint64(limit), uint64(used), nil
}

// AddTraffic атомарно прибавляет байты к traffic_used_bytes.
// Вызывается периодически server'ом из ConnTracker.
func (s *SQLiteStore) AddTraffic(ctx context.Context, username string, n uint64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET traffic_used_bytes = traffic_used_bytes + ? WHERE username = ?`,
		n, username)
	return err
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
