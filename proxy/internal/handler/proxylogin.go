package handler

import (
	"net/http"

	"github.com/ercadev/dirigent/auth"
)

const tokenCookieName = "lotsen_token"

// validProxyToken checks the JWT cookie on the request.
func (h *Handler) validProxyToken(r *http.Request) bool {
	if len(h.jwtSecret) == 0 {
		return true // no secret configured → open
	}
	c, err := r.Cookie(tokenCookieName)
	if err != nil {
		return false
	}
	_, err = auth.ValidateToken(h.jwtSecret, c.Value)
	return err == nil
}

// serveLoginRedirect redirects the client to the dashboard login page so they
// can authenticate with their passkey and obtain a session cookie.
func (h *Handler) serveLoginRedirect(w http.ResponseWriter, r *http.Request) {
	redirect := r.URL.RequestURI()
	target := "/login?redirect=" + redirect
	http.Redirect(w, r, target, http.StatusFound)
}
