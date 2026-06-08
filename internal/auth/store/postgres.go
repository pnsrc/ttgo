package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresStore хранит пользователей в PostgreSQL.
//
// Схема:
//
//	CREATE TABLE IF NOT EXISTS users (
//	    username TEXT PRIMARY KEY,
//	    password TEXT NOT NULL
//	);
type PostgresStore struct {
	db   *sql.DB
	stmt *sql.Stmt
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			username    TEXT PRIMARY KEY,
			password    TEXT NOT NULL,
			max_devices INTEGER NOT NULL DEFAULT 0
		)
	`); err != nil {
		return nil, fmt.Errorf("postgres migrate: %w", err)
	}
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS max_devices INTEGER NOT NULL DEFAULT 0`)
	stmt, err := db.Prepare(`SELECT password FROM users WHERE username = $1`)
	if err != nil {
		return nil, fmt.Errorf("postgres prepare: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	return &PostgresStore{db: db, stmt: stmt}, nil
}

func (s *PostgresStore) GetPassword(ctx context.Context, username string) (string, error) {
	var pw string
	err := s.stmt.QueryRowContext(ctx, username).Scan(&pw)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return pw, err
}

// GetMaxDevices implements auth.DeviceLimitStore.
func (s *PostgresStore) GetMaxDevices(ctx context.Context, username string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT max_devices FROM users WHERE username = $1`, username,
	).Scan(&n)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return n, err
}

func (s *PostgresStore) Close() error {
	s.stmt.Close()
	return s.db.Close()
}
