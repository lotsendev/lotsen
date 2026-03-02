package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUserStore_CreateUser(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateUser("alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 || users[0].Username != "alice" {
		t.Fatalf("want [alice], got %#v", users)
	}
}

func TestUserStore_CreateUser_Duplicate(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateUser("eve"); err != nil {
		t.Fatalf("CreateUser first: %v", err)
	}

	err := s.CreateUser("eve")
	if !errors.Is(err, ErrUserExists) {
		t.Fatalf("want ErrUserExists, got %v", err)
	}
}

func TestUserStore_HasAnyUser(t *testing.T) {
	s := newTestStore(t)

	has, err := s.HasAnyUser()
	if err != nil {
		t.Fatalf("HasAnyUser: %v", err)
	}
	if has {
		t.Error("want false for empty store")
	}

	if err := s.CreateUser("dave"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	has, err = s.HasAnyUser()
	if err != nil {
		t.Fatalf("HasAnyUser after insert: %v", err)
	}
	if !has {
		t.Error("want true after adding a user")
	}
}

func TestUserStore_DeleteUser(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateUser("grace"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := s.DeleteUser("grace"); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	users, _ := s.ListUsers()
	if len(users) != 0 {
		t.Fatal("want empty list after delete")
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

	if err := s.CreateUser("zoe"); err != nil {
		t.Fatalf("CreateUser zoe: %v", err)
	}
	if err := s.CreateUser("amy"); err != nil {
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
