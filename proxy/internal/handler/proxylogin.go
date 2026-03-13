package handler

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/lotsendev/lotsen/auth"
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
	query := url.Values{}
	query.Set("redirect", r.URL.RequestURI())
	target := "/login?" + query.Encode()

	if h.dashboardAuth != nil && h.authCookieDomain != "" {
		host := r.Host
		if bare, _, err := net.SplitHostPort(host); err == nil {
			host = bare
		}
		host = normalizeDomain(host)
		if host != h.dashboardAuth.Domain {
			query.Set("redirect", requestAbsoluteURL(r))
			target = (&url.URL{
				Scheme:   "https",
				Host:     h.dashboardAuth.Domain,
				Path:     "/login",
				RawQuery: query.Encode(),
			}).String()
		}
	}

	http.Redirect(w, r, target, http.StatusFound)
}

func requestAbsoluteURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			candidate := strings.TrimSpace(parts[0])
			if candidate == "http" || candidate == "https" {
				scheme = candidate
			}
		}
	}

	return (&url.URL{Scheme: scheme, Host: r.Host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}).String()
}
