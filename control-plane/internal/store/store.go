package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ErrNotFound is returned when a deployment with the given ID does not exist.
var ErrNotFound = errors.New("deployment not found")

// ErrDuplicateName is returned by Create when a deployment with the same name already exists.
var ErrDuplicateName = errors.New("deployment name already in use")

// Status represents the lifecycle state of a deployment.
type Status string

const (
	StatusIdle      Status = "idle"
	StatusDeploying Status = "deploying"
	StatusHealthy   Status = "healthy"
	StatusFailed    Status = "failed"
)

// Deployment holds the full configuration and runtime state of a container deployment.
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

// JSONStore persists deployments as a JSON array on disk.
// It is safe for concurrent use.
type JSONStore struct {
	mu   sync.RWMutex
	path string
	data map[string]Deployment
}

// NewJSONStore opens or creates the JSON store at path.
// Existing state is loaded into memory on startup.
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

	s := &JSONStore{
		path: path,
		data: make(map[string]Deployment),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *JSONStore) load() error {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: open %s: %w", s.path, err)
	}
	defer f.Close()

	var deployments []Deployment
	if err := json.NewDecoder(f).Decode(&deployments); err != nil {
		return fmt.Errorf("store: decode %s: %w", s.path, err)
	}

	for _, d := range deployments {
		s.data[d.ID] = d
	}

	return nil
}

// persist writes the current state to disk atomically.
// Callers must hold s.mu (write lock) before calling.
func (s *JSONStore) persist() error {
	deployments := make([]Deployment, 0, len(s.data))
	for _, d := range s.data {
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
func (s *JSONStore) List() []Deployment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Deployment, 0, len(s.data))
	for _, d := range s.data {
		result = append(result, d)
	}

	return result
}

// Get returns the deployment with the given ID.
// Returns ErrNotFound if no such deployment exists.
func (s *JSONStore) Get(id string) (Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.data[id]
	if !ok {
		return Deployment{}, ErrNotFound
	}
	return d, nil
}

// Create persists a new deployment and returns it.
// Returns ErrDuplicateName if a deployment with the same name already exists.
func (s *JSONStore) Create(d Deployment) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.data {
		if existing.Name == d.Name {
			return Deployment{}, ErrDuplicateName
		}
	}

	s.data[d.ID] = d

	if err := s.persist(); err != nil {
		delete(s.data, d.ID)
		return Deployment{}, err
	}

	return d, nil
}

// Update replaces the stored deployment and persists to disk.
// Returns ErrNotFound if no deployment with that ID exists.
func (s *JSONStore) Update(d Deployment) (Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev, ok := s.data[d.ID]
	if !ok {
		return Deployment{}, ErrNotFound
	}

	s.data[d.ID] = d

	if err := s.persist(); err != nil {
		s.data[d.ID] = prev
		return Deployment{}, err
	}

	return d, nil
}

// Delete removes the deployment with the given ID.
// Returns ErrNotFound if no such deployment exists.
func (s *JSONStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev, exists := s.data[id]
	if !exists {
		return ErrNotFound
	}

	delete(s.data, id)

	if err := s.persist(); err != nil {
		s.data[id] = prev
		return err
	}

	return nil
}
