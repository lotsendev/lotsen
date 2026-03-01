package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCreateAndValidateToken(t *testing.T) {
	secret := []byte("testsecret")
	token, err := CreateToken(secret, "alice", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	claims, err := ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.Username != "alice" {
		t.Errorf("want username alice, got %s", claims.Username)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	token, err := CreateToken([]byte("correct"), "bob", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	_, err = ValidateToken([]byte("wrong"), token)
	if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("want ErrTokenInvalid, got %v", err)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	secret := []byte("secret")
	token, err := CreateToken(secret, "carol", -time.Second)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	_, err = ValidateToken(secret, token)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("want ErrTokenExpired, got %v", err)
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	secret := []byte("secret")

	for _, tok := range []string{"", "notavalidtoken", "a.b"} {
		_, err := ValidateToken(secret, tok)
		if !errors.Is(err, ErrTokenInvalid) {
			t.Errorf("token %q: want ErrTokenInvalid, got %v", tok, err)
		}
	}
}

func TestValidateToken_TamperedPayload(t *testing.T) {
	secret := []byte("secret")
	token, err := CreateToken(secret, "dave", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	// Tamper with the payload part (middle segment).
	parts := strings.SplitN(token, ".", 3)
	parts[1] = strings.ToUpper(parts[1])
	tampered := strings.Join(parts, ".")

	_, err = ValidateToken(secret, tampered)
	if err == nil {
		t.Error("want error for tampered token, got nil")
	}
}
