package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/ercadev/dirigent/auth"
)

func (h *Handler) listUsers(w http.ResponseWriter, _ *http.Request) {
	if h.authStore == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	users, err := h.authStore.ListUsers()
	if err != nil {
		log.Printf("users: list: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type userResponse struct {
		Username string `json:"username"`
	}

	response := make([]userResponse, 0, len(users))
	for _, user := range users {
		response = append(response, userResponse{Username: user.Username})
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": response})
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)

	var body struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(body.Username)
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	if err := h.authStore.CreateUser(username); err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			http.Error(w, "user already exists", http.StatusConflict)
			return
		}
		log.Printf("users: create: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"username": username})
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	username := strings.TrimSpace(r.PathValue("username"))
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	if err := h.authStore.DeleteUser(username); err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		log.Printf("users: delete: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
