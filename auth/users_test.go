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

func TestUserStore_CreateUser_Duplicate(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateUser("eve", "pass1"); err != nil {
		t.Fatalf("CreateUser first: %v", err)
	}

	err := s.CreateUser("eve", "pass2")
	if !errors.Is(err, ErrUserExists) {
		t.Fatalf("want ErrUserExists, got %v", err)
	}

	if err := s.Authenticate("eve", "pass1"); err != nil {
		t.Fatalf("Authenticate with original password: %v", err)
	}
}

func TestUserStore_UpdatePassword_UserNotFound(t *testing.T) {
	s := newTestStore(t)

	err := s.UpdatePassword("missing", "new-pass")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_UpdatePassword(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateUser("frank", "old-pass"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := s.UpdatePassword("frank", "new-pass"); err != nil {
		t.Fatalf("UpdatePassword: %v", err)
	}

	if err := s.Authenticate("frank", "new-pass"); err != nil {
		t.Fatalf("Authenticate with new password: %v", err)
	}

	err := s.Authenticate("frank", "old-pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestUserStore_DeleteUser(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateUser("grace", "pass"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := s.DeleteUser("grace"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	err := s.Authenticate("grace", "pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials after delete, got %v", err)
	}
}

func TestUserStore_DeleteUser_UserNotFound(t *testing.T) {
	s := newTestStore(t)

	err := s.DeleteUser("missing")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_ListUsers(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateUser("zoe", "pass"); err != nil {
		t.Fatalf("CreateUser zoe: %v", err)
	}
	if err := s.CreateUser("amy", "pass"); err != nil {
		t.Fatalf("CreateUser amy: %v", err)
	}

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("want 2 users, got %d", len(users))
	}

	if users[0].Username != "amy" || users[1].Username != "zoe" {
		t.Fatalf("want users sorted by username, got %#v", users)
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
