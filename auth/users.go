package auth

import (
	"database/sql"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// ErrInvalidCredentials is returned when the username or password is incorrect.
var ErrInvalidCredentials = errors.New("invalid credentials")

// UserStore persists user credentials in SQLite.
type UserStore struct {
	db *sql.DB
}

// NewUserStore opens (or creates) the SQLite database at dbPath and ensures the
// users table exists. dbPath must be a non-empty, absolute file path.
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
			password_hash TEXT    NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("auth: create users table: %w", err)
	}

	return &UserStore{db: db}, nil
}

// Close releases the underlying database connection.
func (s *UserStore) Close() error {
	return s.db.Close()
}

// Authenticate checks username and password. Returns ErrInvalidCredentials on mismatch.
func (s *UserStore) Authenticate(username, password string) error {
	var hash string
	err := s.db.QueryRow(
		"SELECT password_hash FROM users WHERE username = ?", username,
	).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		// Perform a dummy bcrypt to prevent timing-based username enumeration.
		bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidhashpadding000000000000000000000000000000000000"), []byte(password)) //nolint:errcheck
		return ErrInvalidCredentials
	}
	if err != nil {
		return fmt.Errorf("auth: query user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// SetPassword creates or replaces the user's bcrypt-hashed password.
func (s *UserStore) SetPassword(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO users (username, password_hash) VALUES (?, ?)
		ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash
	`, username, string(hash))
	if err != nil {
		return fmt.Errorf("auth: upsert user: %w", err)
	}
	return nil
}

// HasUsers reports whether at least one user exists in the store.
func (s *UserStore) HasUsers() (bool, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false, fmt.Errorf("auth: count users: %w", err)
	}
	return count > 0, nil
}
