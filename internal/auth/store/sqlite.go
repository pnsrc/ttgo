package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
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
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			username TEXT PRIMARY KEY,
			password TEXT NOT NULL
		)
	`); err != nil {
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}
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

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
