package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ercadev/dirigent/auth"
)

const (
	apiTokenCookieName = "dirigent_token"
	apiTokenExpiry     = 24 * time.Hour
)

// login handles POST /auth/login.
// It validates credentials and sets a signed JWT cookie on success.
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil || len(h.jwtSecret) == 0 {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	body.Username = strings.TrimSpace(body.Username)
	body.Password = strings.TrimSpace(body.Password)

	if body.Username == "" || body.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	if err := h.authStore.Authenticate(body.Username, body.Password); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		log.Printf("auth: authenticate: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	token, err := auth.CreateToken(h.jwtSecret, body.Username, apiTokenExpiry)
	if err != nil {
		log.Printf("auth: create token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     apiTokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(apiTokenExpiry.Seconds()),
	})

	writeJSON(w, http.StatusOK, map[string]string{"username": body.Username})
}

// logout handles POST /auth/logout by clearing the session cookie.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     apiTokenCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

// me handles GET /auth/me, returning the currently authenticated user.
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	if len(h.jwtSecret) == 0 {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	claims, err := tokenFromRequest(r, h.jwtSecret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"username": claims.Username})
}

// requireAuth is a middleware that validates the JWT cookie.
// When no authStore is configured it is a no-op (open access).
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.authStore == nil || len(h.jwtSecret) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		if _, err := tokenFromRequest(r, h.jwtSecret); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// tokenFromRequest extracts and validates the JWT from the request cookie.
func tokenFromRequest(r *http.Request, secret []byte) (*auth.Claims, error) {
	c, err := r.Cookie(apiTokenCookieName)
	if err != nil {
		return nil, err
	}
	return auth.ValidateToken(secret, c.Value)
}
