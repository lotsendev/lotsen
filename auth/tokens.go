package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrTokenExpired is returned when a JWT token has passed its expiry time.
var ErrTokenExpired = errors.New("token expired")

// ErrTokenInvalid is returned when a JWT token is malformed or has an invalid signature.
var ErrTokenInvalid = errors.New("token invalid")

const jwtHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" // base64url({"alg":"HS256","typ":"JWT"})

// Claims holds the payload of a Dirigent JWT token.
type Claims struct {
	Username  string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// CreateToken signs a new HS256 JWT for username with the given expiry duration.
func CreateToken(secret []byte, username string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Username:  username,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(expiry).Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("auth: marshal claims: %w", err)
	}

	data := jwtHeader + "." + base64.RawURLEncoding.EncodeToString(payload)
	sig := signHS256(secret, data)

	return data + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// ValidateToken parses and verifies a HS256 JWT, returning its claims on success.
func ValidateToken(secret []byte, token string) (*Claims, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, ErrTokenInvalid
	}

	data := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	expected := signHS256(secret, data)
	if !hmac.Equal(sig, expected) {
		return nil, ErrTokenInvalid
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrTokenInvalid
	}

	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrTokenInvalid
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

func signHS256(secret []byte, data string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
