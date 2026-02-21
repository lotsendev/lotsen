package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/ercadev/dirigent/internal/store"
)

// Store is the persistence interface required by the API handlers.
type Store interface {
	List() ([]store.Deployment, error)
	Get(id string) (store.Deployment, error)
	Create(d store.Deployment) (store.Deployment, error)
	Update(d store.Deployment) (store.Deployment, error)
	Delete(id string) error
}

// Handler holds the dependencies for the API layer.
type Handler struct {
	store Store
}

// New creates a Handler backed by the given store.
func New(s Store) *Handler {
	return &Handler{store: s}
}

// RegisterRoutes wires all deployment endpoints into mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/deployments", h.listDeployments)
	mux.HandleFunc("POST /api/deployments", h.createDeployment)
	mux.HandleFunc("GET /api/deployments/{id}", h.getDeployment)
	mux.HandleFunc("DELETE /api/deployments/{id}", h.deleteDeployment)
}

func (h *Handler) listDeployments(w http.ResponseWriter, r *http.Request) {
	deployments, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to list deployments", http.StatusInternalServerError)
		return
	}
	if deployments == nil {
		deployments = []store.Deployment{}
	}
	writeJSON(w, http.StatusOK, deployments)
}

func (h *Handler) getDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	d, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get deployment", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, d)
}

func (h *Handler) createDeployment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string            `json:"name"`
		Image   string            `json:"image"`
		Envs    map[string]string `json:"envs"`
		Ports   []string          `json:"ports"`
		Volumes []string          `json:"volumes"`
		Domain  string            `json:"domain"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Name == "" || body.Image == "" {
		http.Error(w, "name and image are required", http.StatusBadRequest)
		return
	}

	id, err := newID()
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}

	if body.Envs == nil {
		body.Envs = map[string]string{}
	}
	if body.Ports == nil {
		body.Ports = []string{}
	}
	if body.Volumes == nil {
		body.Volumes = []string{}
	}

	d := store.Deployment{
		ID:      id,
		Name:    body.Name,
		Image:   body.Image,
		Envs:    body.Envs,
		Ports:   body.Ports,
		Volumes: body.Volumes,
		Domain:  body.Domain,
		Status:  store.StatusDeploying,
	}

	created, err := h.store.Create(d)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateName) {
			http.Error(w, "deployment name already in use", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create deployment", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) deleteDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := h.store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "deployment not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete deployment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: encode: %v", err)
	}
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("newID: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
