package auth

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthnUser implements webauthn.User for a Lotsen user.
type WebAuthnUser struct {
	username    string
	credentials []webauthn.Credential
}

// NewWebAuthnUser returns a WebAuthnUser with no credentials. Used for
// transient registration ceremonies where credentials aren't needed yet.
func NewWebAuthnUser(username string) *WebAuthnUser {
	return &WebAuthnUser{username: username}
}

func (u *WebAuthnUser) WebAuthnID() []byte {
	h := sha256.Sum256([]byte(u.username))
	return h[:]
}

func (u *WebAuthnUser) WebAuthnName() string { return u.username }

func (u *WebAuthnUser) WebAuthnDisplayName() string { return u.username }

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// PasskeyInfo is the public view of a stored passkey (no private key material).
type PasskeyInfo struct {
	ID         string
	DeviceName string
	CreatedAt  time.Time
}

// GetWebAuthnUser loads the user record and all their passkey credentials.
func (s *UserStore) GetWebAuthnUser(username string) (*WebAuthnUser, error) {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count); err != nil {
		return nil, fmt.Errorf("auth: get webauthn user: %w", err)
	}
	if count == 0 {
		return nil, ErrUserNotFound
	}

	rows, err := s.db.Query(
		"SELECT data FROM webauthn_credentials WHERE username = ?", username,
	)
	if err != nil {
		return nil, fmt.Errorf("auth: load credentials: %w", err)
	}
	defer rows.Close()

	var creds []webauthn.Credential
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("auth: scan credential: %w", err)
		}
		var cred webauthn.Credential
		if err := json.Unmarshal(raw, &cred); err != nil {
			return nil, fmt.Errorf("auth: unmarshal credential: %w", err)
		}
		creds = append(creds, cred)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: iterate credentials: %w", err)
	}

	return &WebAuthnUser{username: username, credentials: creds}, nil
}

// SavePasskey persists a new WebAuthn credential for a user.
func (s *UserStore) SavePasskey(username string, cred *webauthn.Credential, deviceName string) error {
	raw, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("auth: marshal credential: %w", err)
	}

	credID := fmt.Sprintf("%x", cred.ID)

	_, err = s.db.Exec(
		"INSERT INTO webauthn_credentials (id, username, data, device_name, created_at) VALUES (?, ?, ?, ?, ?)",
		credID, username, raw, deviceName, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("auth: save passkey: %w", err)
	}
	return nil
}

// ListPasskeys returns the public-facing passkey list for a user.
func (s *UserStore) ListPasskeys(username string) ([]PasskeyInfo, error) {
	rows, err := s.db.Query(
		"SELECT id, device_name, created_at FROM webauthn_credentials WHERE username = ? ORDER BY created_at ASC",
		username,
	)
	if err != nil {
		return nil, fmt.Errorf("auth: list passkeys: %w", err)
	}
	defer rows.Close()

	var out []PasskeyInfo
	for rows.Next() {
		var info PasskeyInfo
		var ts int64
		if err := rows.Scan(&info.ID, &info.DeviceName, &ts); err != nil {
			return nil, fmt.Errorf("auth: scan passkey: %w", err)
		}
		info.CreatedAt = time.Unix(ts, 0)
		out = append(out, info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth: iterate passkeys: %w", err)
	}
	return out, nil
}

// DeletePasskey removes a passkey by its hex-encoded ID, scoped to a user.
func (s *UserStore) DeletePasskey(credID, username string) error {
	res, err := s.db.Exec(
		"DELETE FROM webauthn_credentials WHERE id = ? AND username = ?", credID, username,
	)
	if err != nil {
		return fmt.Errorf("auth: delete passkey: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("auth: delete passkey rows affected: %w", err)
	}
	if affected == 0 {
		return errors.New("passkey not found")
	}
	return nil
}

// UpdatePasskeySignCount updates the sign counter for a credential identified
// by its raw (binary) ID.
func (s *UserStore) UpdatePasskeySignCount(credID []byte, count uint32) error {
	hexID := fmt.Sprintf("%x", credID)

	// Fetch the current data blob, update the count, and write back.
	var raw []byte
	if err := s.db.QueryRow(
		"SELECT data FROM webauthn_credentials WHERE id = ?", hexID,
	).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // credential may have been deleted; ignore
		}
		return fmt.Errorf("auth: fetch credential for sign count update: %w", err)
	}

	var cred webauthn.Credential
	if err := json.Unmarshal(raw, &cred); err != nil {
		return fmt.Errorf("auth: unmarshal credential for sign count update: %w", err)
	}
	cred.Authenticator.SignCount = count

	updated, err := json.Marshal(cred)
	if err != nil {
		return fmt.Errorf("auth: marshal credential after sign count update: %w", err)
	}

	_, err = s.db.Exec(
		"UPDATE webauthn_credentials SET data = ? WHERE id = ?", updated, hexID,
	)
	if err != nil {
		return fmt.Errorf("auth: update sign count: %w", err)
	}
	return nil
}
