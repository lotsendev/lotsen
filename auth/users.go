package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// ErrUserNotFound is returned when a user does not exist.
var ErrUserNotFound = errors.New("user not found")

// ErrUserExists is returned when trying to create a duplicate user.
var ErrUserExists = errors.New("user already exists")

type User struct {
	Username string
}

// UserStore persists user credentials and passkeys in SQLite.
type UserStore struct {
	db *sql.DB
}

// NewUserStore opens (or creates) the SQLite database at dbPath and ensures
// the required tables exist. dbPath must be a non-empty, absolute file path.
func NewUserStore(dbPath string) (*UserStore, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("auth: db path must be non-empty")
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("auth: open db: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			username      TEXT    UNIQUE NOT NULL,
			password_hash TEXT    NOT NULL DEFAULT ''
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("auth: create users table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS webauthn_credentials (
			id          TEXT    PRIMARY KEY,
			username    TEXT    NOT NULL REFERENCES users(username) ON DELETE CASCADE,
			data        BLOB    NOT NULL,
			device_name TEXT    NOT NULL DEFAULT '',
			created_at  INTEGER NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("auth: create webauthn_credentials table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS invite_tokens (
			token       TEXT    PRIMARY KEY,
			expires_at  INTEGER NOT NULL,
			used        INTEGER NOT NULL DEFAULT 0
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("auth: create invite_tokens table: %w", err)
	}

	return &UserStore{db: db}, nil
}

// Close releases the underlying database connection.
func (s *UserStore) Close() error {
	return s.db.Close()
}

// CreateUser inserts a new user with an empty password hash (passkey-only).
func (s *UserStore) CreateUser(username string) error {
	_, err := s.db.Exec("INSERT INTO users (username, password_hash) VALUES (?, '!')", username)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrUserExists
		}
		return fmt.Errorf("auth: create user: %w", err)
	}
	return nil
}

// DeleteUser deletes a user by username.
func (s *UserStore) DeleteUser(username string) error {
	res, err := s.db.Exec("DELETE FROM users WHERE username = ?", username)
	if err != nil {
		return fmt.Errorf("auth: delete user: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: delete user rows affected: %w", err)
	}
	if affected == 0 {
		return ErrUserNotFound
	}

	return nil
}

// ListUsers returns all usernames sorted alphabetically.
func (s *UserStore) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT username FROM users ORDER BY username ASC")
	if err != nil {
		return nil, fmt.Errorf("auth: list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Username); err != nil {
			return nil, fmt.Errorf("auth: scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: iterate users: %w", err)
	}

	return users, nil
}

// HasAnyUser reports whether at least one user exists in the store.
func (s *UserStore) HasAnyUser() (bool, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false, fmt.Errorf("auth: count users: %w", err)
	}
	return count > 0, nil
}
