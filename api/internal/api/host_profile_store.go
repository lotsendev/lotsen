package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

type FileHostProfileStore struct {
	mu   sync.RWMutex
	path string
}

func NewFileHostProfileStore(path string) (*FileHostProfileStore, error) {
	if path == "" {
		return nil, fmt.Errorf("host profile store: path must be non-empty")
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("host profile store: path must be absolute: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("host profile store: create data dir: %w", err)
	}

	return &FileHostProfileStore{path: path}, nil
}

func (s *FileHostProfileStore) Get() (HostProfile, error) {
	var result HostProfile
	err := s.withRLock(func() error {
		profile, err := s.read()
		if err != nil {
			return err
		}
		result = profile
		return nil
	})
	if err != nil {
		return HostProfile{}, err
	}

	return result, nil
}

func (s *FileHostProfileStore) UpdateDisplayName(displayName string) (HostProfile, error) {
	updated := HostProfile{}
	err := s.withLock(func() error {
		profile, err := s.read()
		if err != nil {
			return err
		}
		profile.DisplayName = displayName
		if err := s.persist(profile); err != nil {
			return err
		}
		updated = profile
		return nil
	})
	if err != nil {
		return HostProfile{}, err
	}

	return updated, nil
}

func (s *FileHostProfileStore) withLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lf, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("host profile store: open lock file: %w", err)
	}
	defer lf.Close()

	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("host profile store: acquire lock: %w", err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}

func (s *FileHostProfileStore) withRLock(fn func() error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lf, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("host profile store: open lock file: %w", err)
	}
	defer lf.Close()

	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_SH); err != nil {
		return fmt.Errorf("host profile store: acquire shared lock: %w", err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}

func (s *FileHostProfileStore) read() (HostProfile, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return HostProfile{}, nil
	}
	if err != nil {
		return HostProfile{}, fmt.Errorf("host profile store: open %s: %w", s.path, err)
	}
	defer f.Close()

	var profile HostProfile
	if err := json.NewDecoder(f).Decode(&profile); err != nil {
		return HostProfile{}, fmt.Errorf("host profile store: decode %s: %w", s.path, err)
	}

	return profile, nil
}

func (s *FileHostProfileStore) persist(profile HostProfile) error {
	tmp := s.path + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("host profile store: create temp file: %w", err)
	}

	if err := json.NewEncoder(f).Encode(profile); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("host profile store: encode: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("host profile store: sync temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("host profile store: close temp file: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("host profile store: rename: %w", err)
	}

	return nil
}
