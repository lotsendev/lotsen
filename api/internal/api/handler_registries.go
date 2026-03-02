package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ercadev/dirigent/store"
)

type registryRequest struct {
	Prefix   string `json:"prefix"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) listRegistries(w http.ResponseWriter, _ *http.Request) {
	registries, err := h.store.ListRegistries()
	if err != nil {
		http.Error(w, "failed to list registries", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"registries": registries})
}

func (h *Handler) createRegistry(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)

	var body registryRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	prefix := strings.TrimSpace(body.Prefix)
	username := strings.TrimSpace(body.Username)
	password := strings.TrimSpace(body.Password)
	if prefix == "" || username == "" || password == "" {
		http.Error(w, "prefix, username and password are required", http.StatusBadRequest)
		return
	}

	id, err := newID()
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}

	created, err := h.store.CreateRegistry(id, prefix, username, password)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateRegistryPrefix) {
			http.Error(w, "registry prefix already in use", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create registry", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) updateRegistry(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "registry id is required", http.StatusBadRequest)
		return
	}

	var body registryRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	prefix := strings.TrimSpace(body.Prefix)
	username := strings.TrimSpace(body.Username)
	if prefix == "" || username == "" {
		http.Error(w, "prefix and username are required", http.StatusBadRequest)
		return
	}

	updated, err := h.store.UpdateRegistry(id, prefix, username, strings.TrimSpace(body.Password))
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRegistryNotFound):
			http.Error(w, "registry not found", http.StatusNotFound)
		case errors.Is(err, store.ErrDuplicateRegistryPrefix):
			http.Error(w, "registry prefix already in use", http.StatusConflict)
		default:
			http.Error(w, "failed to update registry", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) deleteRegistry(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "registry id is required", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteRegistry(id); err != nil {
		if errors.Is(err, store.ErrRegistryNotFound) {
			http.Error(w, "registry not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete registry", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
