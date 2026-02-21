package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// Status represents the lifecycle state of a deployment.
type Status string

const (
	StatusIdle      Status = "idle"
	StatusDeploying Status = "deploying"
	StatusHealthy   Status = "healthy"
	StatusFailed    Status = "failed"
)

// Deployment holds the configuration and runtime state of a container deployment.
type Deployment struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Envs    map[string]string `json:"envs"`
	Ports   []string          `json:"ports"`
	Volumes []string          `json:"volumes"`
	Domain  string            `json:"domain"`
	Status  Status            `json:"status"`
}

// Store is the persistence interface required by the reconciler.
type Store interface {
	List() ([]Deployment, error)
	UpdateStatus(id string, status Status) error
}

// JSONStore persists deployments as a JSON array on disk.
// It is safe for concurrent use within a process (sync.RWMutex) and across
// processes (syscall.Flock on a .lock file).
type JSONStore struct {
	mu   sync.RWMutex
	path string
}

// NewJSONStore opens the JSON store at path.
// path must be a non-empty absolute file path.
func NewJSONStore(path string) (*JSONStore, error) {
	if path == "" {
		return nil, fmt.Errorf("store: path must be non-empty")
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("store: path must be absolute: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: create data dir: %w", err)
	}
	return &JSONStore{path: path}, nil
}

func (s *JSONStore) withLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lf, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("store: open lock file: %w", err)
	}
	defer lf.Close()

	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("store: acquire lock: %w", err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}

func (s *JSONStore) withRLock(fn func() error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lf, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("store: open lock file: %w", err)
	}
	defer lf.Close()

	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_SH); err != nil {
		return fmt.Errorf("store: acquire shared lock: %w", err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}

func (s *JSONStore) read() (map[string]Deployment, error) {
	data := make(map[string]Deployment)

	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return data, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", s.path, err)
	}
	defer f.Close()

	var deployments []Deployment
	if err := json.NewDecoder(f).Decode(&deployments); err != nil {
		return nil, fmt.Errorf("store: decode %s: %w", s.path, err)
	}

	for _, d := range deployments {
		data[d.ID] = d
	}

	return data, nil
}

func (s *JSONStore) persist(data map[string]Deployment) error {
	deployments := make([]Deployment, 0, len(data))
	for _, d := range data {
		deployments = append(deployments, d)
	}

	tmp := s.path + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("store: create temp file: %w", err)
	}

	if err := json.NewEncoder(f).Encode(deployments); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("store: encode: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("store: sync temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("store: close temp file: %w", err)
	}

	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("store: rename: %w", err)
	}

	return nil
}

// List returns a snapshot of all deployments.
func (s *JSONStore) List() ([]Deployment, error) {
	var result []Deployment
	err := s.withRLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		result = make([]Deployment, 0, len(data))
		for _, d := range data {
			result = append(result, d)
		}
		return nil
	})
	return result, err
}

// UpdateStatus sets the status of the deployment with the given ID.
// If the deployment no longer exists in the store it is a no-op.
func (s *JSONStore) UpdateStatus(id string, status Status) error {
	return s.withLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		d, ok := data[id]
		if !ok {
			return nil // deployment was deleted; ignore
		}
		d.Status = status
		data[id] = d
		return s.persist(data)
	})
}
