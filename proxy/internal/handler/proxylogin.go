package handler

import (
	_ "embed"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/ercadev/dirigent/auth"
)

//go:embed login.html
var loginPageHTML string

var loginTmpl = template.Must(template.New("login").Parse(loginPageHTML))

const (
	tokenCookieName = "lotsen_token"
	tokenExpiry     = 24 * time.Hour
)

type loginPageData struct {
	Redirect string
	Error    string
}

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

// serveLoginPage renders the embedded login page with an optional error message.
func (h *Handler) serveLoginPage(w http.ResponseWriter, r *http.Request, errMsg string) {
	redirect := r.URL.RequestURI()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	if err := loginTmpl.Execute(w, loginPageData{Redirect: redirect, Error: errMsg}); err != nil {
		log.Printf("proxy: render login page: %v", err)
	}
}

// proxyLogin handles POST /__lotsen/login for private-deployment authentication.
func (h *Handler) proxyLogin(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil || len(h.jwtSecret) == 0 {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	redirect := r.FormValue("redirect")
	if redirect == "" {
		redirect = "/"
	}

	if username == "" || password == "" {
		h.serveLoginPage(w, r, "Username and password are required.")
		return
	}

	if err := h.authStore.Authenticate(username, password); err != nil {
		h.serveLoginPage(w, r, "Invalid username or password.")
		return
	}

	token, err := auth.CreateToken(h.jwtSecret, username, tokenExpiry)
	if err != nil {
		log.Printf("proxy: create token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(tokenExpiry.Seconds()),
	})

	http.Redirect(w, r, redirect, http.StatusSeeOther)
}
