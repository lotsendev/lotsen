package auth

import (
	"errors"
	"fmt"
	"time"
)

// ErrInviteNotFound is returned when a token does not exist.
var ErrInviteNotFound = errors.New("invite token not found")

// ErrInviteExpired is returned when the token has passed its expiry.
var ErrInviteExpired = errors.New("invite token expired")

// ErrInviteUsed is returned when the token has already been consumed.
var ErrInviteUsed = errors.New("invite token already used")

// CreateInviteToken stores a new invite token with the given expiry time.
func (s *UserStore) CreateInviteToken(token string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		"INSERT INTO invite_tokens (token, expires_at, used) VALUES (?, ?, 0)",
		token, expiresAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("auth: create invite token: %w", err)
	}
	return nil
}

// ValidateInviteToken checks that the token exists, is not expired, and has
// not been used. Returns a typed error for each failure mode.
func (s *UserStore) ValidateInviteToken(token string) error {
	var expiresAt int64
	var used int
	err := s.db.QueryRow(
		"SELECT expires_at, used FROM invite_tokens WHERE token = ?", token,
	).Scan(&expiresAt, &used)
	if err != nil {
		return ErrInviteNotFound
	}
	if used != 0 {
		return ErrInviteUsed
	}
	if time.Now().Unix() > expiresAt {
		return ErrInviteExpired
	}
	return nil
}

// ConsumeInviteToken marks the token as used (called after successful registration).
func (s *UserStore) ConsumeInviteToken(token string) error {
	res, err := s.db.Exec(
		"UPDATE invite_tokens SET used = 1 WHERE token = ?", token,
	)
	if err != nil {
		return fmt.Errorf("auth: consume invite token: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: consume invite token rows affected: %w", err)
	}
	if affected == 0 {
		return ErrInviteNotFound
	}
	return nil
}

// CleanupExpiredTokens removes expired (and used) tokens. Safe to call at startup.
func (s *UserStore) CleanupExpiredTokens() error {
	_, err := s.db.Exec(
		"DELETE FROM invite_tokens WHERE expires_at < ? OR used = 1",
		time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("auth: cleanup expired tokens: %w", err)
	}
	return nil
}
