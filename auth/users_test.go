package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUserStore_AuthenticateSuccess(t *testing.T) {
	s := newTestStore(t)

	if err := s.SetPassword("alice", "secret123"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	if err := s.Authenticate("alice", "secret123"); err != nil {
		t.Errorf("Authenticate: want nil, got %v", err)
	}
}

func TestUserStore_AuthenticateWrongPassword(t *testing.T) {
	s := newTestStore(t)
	if err := s.SetPassword("bob", "correct"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	err := s.Authenticate("bob", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestUserStore_AuthenticateUnknownUser(t *testing.T) {
	s := newTestStore(t)

	err := s.Authenticate("nobody", "pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("want ErrInvalidCredentials for unknown user, got %v", err)
	}
}

func TestUserStore_SetPasswordUpserts(t *testing.T) {
	s := newTestStore(t)
	if err := s.SetPassword("carol", "first"); err != nil {
		t.Fatalf("SetPassword first: %v", err)
	}
	if err := s.SetPassword("carol", "second"); err != nil {
		t.Fatalf("SetPassword second: %v", err)
	}

	if err := s.Authenticate("carol", "second"); err != nil {
		t.Errorf("Authenticate with new password: %v", err)
	}
	if err := s.Authenticate("carol", "first"); !errors.Is(err, ErrInvalidCredentials) {
		t.Error("old password must no longer work")
	}
}

func TestUserStore_HasUsers(t *testing.T) {
	s := newTestStore(t)

	has, err := s.HasUsers()
	if err != nil {
		t.Fatalf("HasUsers: %v", err)
	}
	if has {
		t.Error("want false for empty store")
	}

	if err := s.SetPassword("dave", "pw"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}

	has, err = s.HasUsers()
	if err != nil {
		t.Fatalf("HasUsers after insert: %v", err)
	}
	if !has {
		t.Error("want true after adding a user")
	}
}

func TestNewUserStore_EmptyPath(t *testing.T) {
	_, err := NewUserStore("")
	if err == nil {
		t.Error("want error for empty path")
	}
}

func newTestStore(t *testing.T) *UserStore {
	t.Helper()
	dir := t.TempDir()
	s, err := NewUserStore(filepath.Join(dir, "users.db"))
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(filepath.Join(dir, "users.db"))
	})
	return s
}
