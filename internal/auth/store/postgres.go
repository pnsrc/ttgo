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
			username TEXT PRIMARY KEY,
			password TEXT NOT NULL
		)
	`); err != nil {
		return nil, fmt.Errorf("postgres migrate: %w", err)
	}
	// Prepare для быстрых повторных запросов
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

func (s *PostgresStore) Close() error {
	s.stmt.Close()
	return s.db.Close()
}
