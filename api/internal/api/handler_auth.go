package api

import (
	"log"
	"net/http"
	"time"

	"github.com/ercadev/lotsen/auth"
)

const (
	apiTokenCookieName = "lotsen_token"
	apiTokenExpiry     = 24 * time.Hour
)

// logout handles POST /auth/logout by clearing the session cookie.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, h.newSessionCookie(r, "", -1))
	w.WriteHeader(http.StatusNoContent)
}

// me handles GET /auth/me, returning the currently authenticated user.
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	if h.dashboardAccessMode() == DashboardAccessModeWAFOnly {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

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
		if h.dashboardAccessMode() == DashboardAccessModeWAFOnly {
			next.ServeHTTP(w, r)
			return
		}

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

// issueSessionCookie mints a JWT for username and sets the session cookie.
// Returns true on success; on failure writes the error response and returns false.
func (h *Handler) issueSessionCookie(w http.ResponseWriter, r *http.Request, username string) bool {
	token, err := auth.CreateToken(h.jwtSecret, username, apiTokenExpiry)
	if err != nil {
		log.Printf("auth: create token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return false
	}

	http.SetCookie(w, h.newSessionCookie(r, token, int(apiTokenExpiry.Seconds())))
	return true
}

func (h *Handler) newSessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	c := &http.Cookie{
		Name:     apiTokenCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	}
	if h.authCookieDomain != "" {
		c.Domain = h.authCookieDomain
	}
	return c
}
