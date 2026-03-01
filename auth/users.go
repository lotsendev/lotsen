package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// ErrInvalidCredentials is returned when the username or password is incorrect.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrUserNotFound is returned when a user does not exist.
var ErrUserNotFound = errors.New("user not found")

// ErrUserExists is returned when trying to create a duplicate user.
var ErrUserExists = errors.New("user already exists")

type User struct {
	Username string
}

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

// CreateUser creates a new user with a bcrypt-hashed password.
func (s *UserStore) CreateUser(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}

	_, err = s.db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, string(hash))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrUserExists
		}
		return fmt.Errorf("auth: create user: %w", err)
	}

	return nil
}

// UpdatePassword updates the password hash for an existing user.
func (s *UserStore) UpdatePassword(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("auth: hash password: %w", err)
	}

	res, err := s.db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", string(hash), username)
	if err != nil {
		return fmt.Errorf("auth: update password: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: update password rows affected: %w", err)
	}
	if affected == 0 {
		return ErrUserNotFound
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

// HasUsers reports whether at least one user exists in the store.
func (s *UserStore) HasUsers() (bool, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false, fmt.Errorf("auth: count users: %w", err)
	}
	return count > 0, nil
}
