package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// ErrNotFound is returned when a deployment with the given ID does not exist.
var ErrNotFound = errors.New("deployment not found")

// ErrDuplicateName is returned by Create when a deployment with the same name already exists.
var ErrDuplicateName = errors.New("deployment name already in use")

// ErrRegistryNotFound is returned when a registry with the given ID does not exist.
var ErrRegistryNotFound = errors.New("registry not found")

// ErrDuplicateRegistryPrefix is returned when a registry with the same prefix already exists.
var ErrDuplicateRegistryPrefix = errors.New("registry prefix already in use")

// Status represents the lifecycle state of a deployment.
type Status string

const (
	StatusIdle      Status = "idle"
	StatusDeploying Status = "deploying"
	StatusHealthy   Status = "healthy"
	StatusFailed    Status = "failed"
)

// StatusReason is a stable machine-readable code that explains why a
// deployment transitioned status.
type StatusReason string

const (
	StatusReasonDeployStartSucceeded      StatusReason = "deploy_start_succeeded"
	StatusReasonRedeployStartSucceeded    StatusReason = "redeploy_start_succeeded"
	StatusReasonRetryStartSucceeded       StatusReason = "retry_start_succeeded"
	StatusReasonRetryRecoveredRunning     StatusReason = "retry_recovered_running"
	StatusReasonDockerUnavailable         StatusReason = "docker_unavailable"
	StatusReasonDomainReserved            StatusReason = "domain_reserved"
	StatusReasonDeployStartFailed         StatusReason = "deploy_start_failed"
	StatusReasonRedeployStartFailed       StatusReason = "redeploy_start_failed"
	StatusReasonContainerExited           StatusReason = "container_exited"
	StatusReasonContainerNotRunning       StatusReason = "container_not_running"
	StatusReasonRetryStartFailedTransient StatusReason = "retry_start_failed_transient"
)

// Deployment holds the full configuration and runtime state of a container deployment.
type Deployment struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Envs         map[string]string `json:"envs"`
	Ports        []string          `json:"ports"`
	ProxyPort    int               `json:"proxy_port,omitempty"`
	Volumes      []string          `json:"volumes"`
	Domain       string            `json:"domain"`
	Public       bool              `json:"public,omitempty"`
	PublicSet    bool              `json:"-"`
	ProxyPortSet bool              `json:"-"`
	BasicAuth    *BasicAuthConfig  `json:"basic_auth,omitempty"`
	Security     *SecurityConfig   `json:"security,omitempty"`
	Status       Status            `json:"status"`
	Reason       StatusReason      `json:"reason,omitempty"`
	Error        string            `json:"error,omitempty"`
}

type BasicAuthConfig struct {
	Users []BasicAuthUser `json:"users"`
}

type BasicAuthUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SecurityConfig struct {
	WAFEnabled  bool     `json:"waf_enabled"`
	WAFMode     string   `json:"waf_mode,omitempty"`
	IPDenylist  []string `json:"ip_denylist,omitempty"`
	IPAllowlist []string `json:"ip_allowlist,omitempty"`
	CustomRules []string `json:"custom_rules,omitempty"`
}

type Registry struct {
	ID       string `json:"id"`
	Prefix   string `json:"prefix"`
	Username string `json:"username"`
	Secret   string `json:"secret"`
}

type RegistryEntry struct {
	ID       string `json:"id"`
	Prefix   string `json:"prefix"`
	Username string `json:"username"`
}

type RegistryAuth struct {
	Username string
	Password string
}

type persistedState struct {
	Deployments []Deployment `json:"deployments"`
	Registries  []Registry   `json:"registries,omitempty"`
}

// JSONStore persists deployments as a JSON array on disk.
// It is safe for concurrent use within a process (sync.RWMutex) and across
// processes (syscall.Flock on a .lock file).
type JSONStore struct {
	mu   sync.RWMutex
	path string
}

// NewJSONStore opens or creates the JSON store at path.
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

// withLock acquires an exclusive OS-level file lock and the in-process mutex,
// then calls fn. Both are released before returning.
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

// withRLock acquires a shared OS-level file lock and the in-process read mutex,
// then calls fn. Both are released before returning.
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

// read loads deployments from disk into a map. Caller must hold the lock.
func (s *JSONStore) readState() (persistedState, error) {
	state := persistedState{
		Deployments: []Deployment{},
		Registries:  []Registry{},
	}

	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return persistedState{}, fmt.Errorf("store: open %s: %w", s.path, err)
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	var asObject persistedState
	if err := decoder.Decode(&asObject); err == nil {
		if asObject.Deployments == nil {
			asObject.Deployments = []Deployment{}
		}
		if asObject.Registries == nil {
			asObject.Registries = []Registry{}
		}
		return asObject, nil
	}

	if _, seekErr := f.Seek(0, 0); seekErr != nil {
		return persistedState{}, fmt.Errorf("store: rewind %s: %w", s.path, seekErr)
	}

	var deployments []Deployment
	if err := json.NewDecoder(f).Decode(&deployments); err != nil {
		return persistedState{}, fmt.Errorf("store: decode %s: %w", s.path, err)
	}

	state.Deployments = deployments
	return state, nil
}

// read loads deployments from disk into a map. Caller must hold the lock.
func (s *JSONStore) read() (map[string]Deployment, error) {
	data := make(map[string]Deployment)

	state, err := s.readState()
	if err != nil {
		return nil, err
	}

	for _, d := range state.Deployments {
		data[d.ID] = d
	}

	return data, nil
}

// persist writes data to disk atomically. Caller must hold the lock.
func (s *JSONStore) persistState(state persistedState) error {
	if state.Deployments == nil {
		state.Deployments = []Deployment{}
	}
	if state.Registries == nil {
		state.Registries = []Registry{}
	}

	tmp := s.path + ".tmp"

	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("store: create temp file: %w", err)
	}

	if err := json.NewEncoder(f).Encode(state); err != nil {
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

// persist writes data to disk atomically. Caller must hold the lock.
func (s *JSONStore) persist(data map[string]Deployment) error {
	deployments := make([]Deployment, 0, len(data))
	for _, d := range data {
		deployments = append(deployments, d)
	}

	state, err := s.readState()
	if err != nil {
		return err
	}
	state.Deployments = deployments

	return s.persistState(state)
}

func (s *JSONStore) ListRegistries() ([]RegistryEntry, error) {
	var result []RegistryEntry
	err := s.withRLock(func() error {
		state, err := s.readState()
		if err != nil {
			return err
		}
		result = make([]RegistryEntry, 0, len(state.Registries))
		for _, r := range state.Registries {
			result = append(result, RegistryEntry{ID: r.ID, Prefix: r.Prefix, Username: r.Username})
		}
		return nil
	})
	return result, err
}

func (s *JSONStore) CreateRegistry(id, prefix, username, password string) (RegistryEntry, error) {
	prefix = normalizeRegistryPrefix(prefix)
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if id == "" || prefix == "" || username == "" || password == "" {
		return RegistryEntry{}, fmt.Errorf("store: registry id, prefix, username and password are required")
	}

	secret, err := s.encryptSecret(password)
	if err != nil {
		return RegistryEntry{}, err
	}

	err = s.withLock(func() error {
		state, err := s.readState()
		if err != nil {
			return err
		}
		for _, existing := range state.Registries {
			if existing.Prefix == prefix {
				return ErrDuplicateRegistryPrefix
			}
		}
		state.Registries = append(state.Registries, Registry{ID: id, Prefix: prefix, Username: username, Secret: secret})
		return s.persistState(state)
	})
	if err != nil {
		return RegistryEntry{}, err
	}

	return RegistryEntry{ID: id, Prefix: prefix, Username: username}, nil
}

func (s *JSONStore) UpdateRegistry(id, prefix, username, password string) (RegistryEntry, error) {
	prefix = normalizeRegistryPrefix(prefix)
	username = strings.TrimSpace(username)
	if id == "" || prefix == "" || username == "" {
		return RegistryEntry{}, fmt.Errorf("store: registry id, prefix, and username are required")
	}

	password = strings.TrimSpace(password)

	err := s.withLock(func() error {
		state, err := s.readState()
		if err != nil {
			return err
		}

		index := -1
		for i, existing := range state.Registries {
			if existing.ID == id {
				index = i
				continue
			}
			if existing.Prefix == prefix {
				return ErrDuplicateRegistryPrefix
			}
		}
		if index == -1 {
			return ErrRegistryNotFound
		}

		updated := state.Registries[index]
		updated.Prefix = prefix
		updated.Username = username
		if password != "" {
			secret, err := s.encryptSecret(password)
			if err != nil {
				return err
			}
			updated.Secret = secret
		}

		state.Registries[index] = updated
		return s.persistState(state)
	})
	if err != nil {
		return RegistryEntry{}, err
	}

	return RegistryEntry{ID: id, Prefix: prefix, Username: username}, nil
}

func (s *JSONStore) DeleteRegistry(id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrRegistryNotFound
	}

	return s.withLock(func() error {
		state, err := s.readState()
		if err != nil {
			return err
		}
		for i, entry := range state.Registries {
			if entry.ID != id {
				continue
			}
			state.Registries = append(state.Registries[:i], state.Registries[i+1:]...)
			return s.persistState(state)
		}
		return ErrRegistryNotFound
	})
}

func (s *JSONStore) ResolveRegistryAuth(imageRef string) (*RegistryAuth, error) {
	imageRef = normalizeImageRef(imageRef)
	if imageRef == "" {
		return nil, nil
	}

	var match *Registry
	err := s.withRLock(func() error {
		state, err := s.readState()
		if err != nil {
			return err
		}
		for i := range state.Registries {
			candidate := state.Registries[i]
			if !registryMatchesImage(candidate.Prefix, imageRef) {
				continue
			}
			if match == nil || len(candidate.Prefix) > len(match.Prefix) {
				copied := candidate
				match = &copied
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if match == nil {
		return nil, nil
	}

	password, err := s.decryptSecret(match.Secret)
	if err != nil {
		return nil, err
	}

	return &RegistryAuth{Username: match.Username, Password: password}, nil
}

func normalizeRegistryPrefix(prefix string) string {
	prefix = strings.TrimSpace(strings.ToLower(prefix))
	prefix = strings.TrimPrefix(prefix, "https://")
	prefix = strings.TrimPrefix(prefix, "http://")
	prefix = strings.TrimSuffix(prefix, "/")
	return prefix
}

func normalizeImageRef(image string) string {
	return strings.TrimSpace(strings.ToLower(image))
}

func registryMatchesImage(prefix, imageRef string) bool {
	if !strings.HasPrefix(imageRef, prefix) {
		return false
	}
	if len(imageRef) == len(prefix) {
		return true
	}
	sep := imageRef[len(prefix)]
	return sep == '/' || sep == ':' || sep == '@'
}

func (s *JSONStore) encryptSecret(secret string) (string, error) {
	block, err := aes.NewCipher(s.secretKey())
	if err != nil {
		return "", fmt.Errorf("store: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("store: create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("store: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(secret), nil)
	payload := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

func (s *JSONStore) decryptSecret(ciphertext string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("store: decode secret: %w", err)
	}

	block, err := aes.NewCipher(s.secretKey())
	if err != nil {
		return "", fmt.Errorf("store: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("store: create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(payload) < nonceSize {
		return "", fmt.Errorf("store: invalid encrypted secret payload")
	}

	nonce := payload[:nonceSize]
	enc := payload[nonceSize:]
	plain, err := gcm.Open(nil, nonce, enc, nil)
	if err != nil {
		return "", fmt.Errorf("store: decrypt secret: %w", err)
	}

	return string(plain), nil
}

func (s *JSONStore) secretKey() []byte {
	keyMaterial := strings.TrimSpace(os.Getenv("LOTSEN_REGISTRY_SECRET"))
	if keyMaterial == "" {
		keyMaterial = strings.TrimSpace(os.Getenv("LOTSEN_JWT_SECRET"))
	}
	if keyMaterial == "" {
		keyMaterial = "dirigent-registry:" + s.path
	}

	sum := sha256.Sum256([]byte(keyMaterial))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	return key
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

// Get returns the deployment with the given ID.
// Returns ErrNotFound if no such deployment exists.
func (s *JSONStore) Get(id string) (Deployment, error) {
	var result Deployment
	err := s.withRLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		d, ok := data[id]
		if !ok {
			return ErrNotFound
		}
		result = d
		return nil
	})
	return result, err
}

// Create persists a new deployment and returns it.
// Returns ErrDuplicateName if a deployment with the same name already exists.
func (s *JSONStore) Create(d Deployment) (Deployment, error) {
	err := s.withLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		for _, existing := range data {
			if existing.Name == d.Name {
				return ErrDuplicateName
			}
		}
		data[d.ID] = d
		return s.persist(data)
	})
	if err != nil {
		return Deployment{}, err
	}
	return d, nil
}

// Update replaces the stored deployment and persists to disk.
// Returns ErrNotFound if no deployment with that ID exists.
func (s *JSONStore) Update(d Deployment) (Deployment, error) {
	err := s.withLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		if _, ok := data[d.ID]; !ok {
			return ErrNotFound
		}
		data[d.ID] = d
		return s.persist(data)
	})
	if err != nil {
		return Deployment{}, err
	}
	return d, nil
}

// Patch merges the non-zero fields of patch into the stored deployment and persists atomically.
// Only image, envs, ports, volumes, domain, basic auth config, status, and error are merged; id and name are immutable.
// Returns ErrNotFound if no deployment with that ID exists.
func (s *JSONStore) Patch(id string, patch Deployment) (Deployment, error) {
	var result Deployment
	err := s.withLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		d, ok := data[id]
		if !ok {
			return ErrNotFound
		}
		if patch.Image != "" {
			d.Image = patch.Image
		}
		if patch.Envs != nil {
			d.Envs = patch.Envs
		}
		if patch.Ports != nil {
			d.Ports = patch.Ports
		}
		if patch.Volumes != nil {
			d.Volumes = patch.Volumes
		}
		if patch.ProxyPortSet {
			d.ProxyPort = patch.ProxyPort
		}
		if patch.Domain != "" {
			d.Domain = patch.Domain
		}
		if patch.PublicSet {
			d.Public = patch.Public
		}
		if patch.BasicAuth != nil {
			d.BasicAuth = patch.BasicAuth
		}
		if patch.Security != nil {
			d.Security = patch.Security
		}
		if patch.Status != "" {
			d.Status = patch.Status
			d.Reason = patch.Reason
			d.Error = patch.Error
		} else if patch.Error != "" {
			d.Error = patch.Error
		}
		data[id] = d
		if err := s.persist(data); err != nil {
			return err
		}
		result = d
		return nil
	})
	if err != nil {
		return Deployment{}, err
	}
	return result, nil
}

// Delete removes the deployment with the given ID.
// Returns ErrNotFound if no such deployment exists.
func (s *JSONStore) Delete(id string) error {
	return s.withLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		if _, exists := data[id]; !exists {
			return ErrNotFound
		}
		delete(data, id)
		return s.persist(data)
	})
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
		d.Reason = ""
		d.Error = ""
		data[id] = d
		return s.persist(data)
	})
}
