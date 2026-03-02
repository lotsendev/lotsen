package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/ercadev/dirigent/auth"
)

const passkeySessionCookie = "lotsen_pk_session"

// passkeySessionStore holds in-flight WebAuthn session data (challenges).
type passkeySessionStore struct {
	mu   sync.Mutex
	data map[string]*webauthn.SessionData
}

func newPasskeySessionStore() *passkeySessionStore {
	s := &passkeySessionStore{data: make(map[string]*webauthn.SessionData)}
	go s.cleanupLoop()
	return s
}

func (s *passkeySessionStore) set(key string, sd *webauthn.SessionData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = sd
}

func (s *passkeySessionStore) get(key string) (*webauthn.SessionData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sd, ok := s.data[key]
	return sd, ok
}

func (s *passkeySessionStore) delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

func (s *passkeySessionStore) cleanupLoop() {
	for range time.Tick(5 * time.Minute) {
		s.mu.Lock()
		s.data = make(map[string]*webauthn.SessionData)
		s.mu.Unlock()
	}
}

// ── Setup (first-run) ──────────────────────────────────────────────────────

// setupAvailable handles GET /auth/setup-available.
func (h *Handler) setupAvailable(w http.ResponseWriter, _ *http.Request) {
	if h.authStore == nil {
		writeJSON(w, http.StatusOK, map[string]bool{"available": false})
		return
	}
	has, err := h.authStore.HasAnyUser()
	if err != nil {
		log.Printf("passkey: check users: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"available": !has})
}

// passkeySetupBegin handles POST /auth/passkey/setup/begin. Body: {username}.
func (h *Handler) passkeySetupBegin(w http.ResponseWriter, r *http.Request) {
	if !h.setupIsAllowed(w) {
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

	creation, session, err := h.webAuthn.BeginRegistration(auth.NewWebAuthnUser(username))
	if err != nil {
		log.Printf("passkey: begin setup: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.challenges.set("setup:"+username, session)
	writeJSON(w, http.StatusOK, creation)
}

// passkeySetupFinish handles POST /auth/passkey/setup/finish?username=<name>.
func (h *Handler) passkeySetupFinish(w http.ResponseWriter, r *http.Request) {
	if !h.setupIsAllowed(w) {
		return
	}

	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		http.Error(w, "username query param is required", http.StatusBadRequest)
		return
	}

	session, ok := h.challenges.get("setup:" + username)
	if !ok {
		http.Error(w, "no pending registration session", http.StatusBadRequest)
		return
	}
	h.challenges.delete("setup:" + username)

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(r.Body)
	if err != nil {
		log.Printf("passkey: parse setup credential: %v", err)
		http.Error(w, "invalid credential response", http.StatusBadRequest)
		return
	}

	cred, err := h.webAuthn.CreateCredential(auth.NewWebAuthnUser(username), *session, parsedResponse)
	if err != nil {
		log.Printf("passkey: finish setup: %v", err)
		http.Error(w, "registration failed", http.StatusBadRequest)
		return
	}

	if err := h.authStore.CreateUser(username); err != nil && !errors.Is(err, auth.ErrUserExists) {
		log.Printf("passkey: create user on setup: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.authStore.SavePasskey(username, cred, ""); err != nil {
		log.Printf("passkey: save passkey on setup: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if !h.issueSessionCookie(w, r, username) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": username})
}

// ── Invite registration ────────────────────────────────────────────────────

// validateInvite handles GET /auth/invite?token=...
func (h *Handler) validateInvite(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false})
		return
	}
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}
	err := h.authStore.ValidateInviteToken(token)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "reason": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true})
}

// passkeyInviteBegin handles POST /auth/passkey/invite/begin. Body: {token, username}.
func (h *Handler) passkeyInviteBegin(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil || h.webAuthn == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var body struct {
		Token    string `json:"token"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(body.Token)
	username := strings.TrimSpace(body.Username)
	if token == "" || username == "" {
		http.Error(w, "token and username are required", http.StatusBadRequest)
		return
	}

	if err := h.authStore.ValidateInviteToken(token); err != nil {
		http.Error(w, "invalid or expired invite token", http.StatusForbidden)
		return
	}

	creation, session, err := h.webAuthn.BeginRegistration(auth.NewWebAuthnUser(username))
	if err != nil {
		log.Printf("passkey: begin invite: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.challenges.set("invite:"+token+":"+username, session)
	writeJSON(w, http.StatusOK, creation)
}

// passkeyInviteFinish handles POST /auth/passkey/invite/finish?token=...&username=...
func (h *Handler) passkeyInviteFinish(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil || h.webAuthn == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if token == "" || username == "" {
		http.Error(w, "token and username query params are required", http.StatusBadRequest)
		return
	}

	if err := h.authStore.ValidateInviteToken(token); err != nil {
		http.Error(w, "invalid or expired invite token", http.StatusForbidden)
		return
	}

	sessionKey := "invite:" + token + ":" + username
	session, ok := h.challenges.get(sessionKey)
	if !ok {
		http.Error(w, "no pending registration session", http.StatusBadRequest)
		return
	}
	h.challenges.delete(sessionKey)

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(r.Body)
	if err != nil {
		log.Printf("passkey: parse invite credential: %v", err)
		http.Error(w, "invalid credential response", http.StatusBadRequest)
		return
	}

	cred, err := h.webAuthn.CreateCredential(auth.NewWebAuthnUser(username), *session, parsedResponse)
	if err != nil {
		log.Printf("passkey: finish invite: %v", err)
		http.Error(w, "registration failed", http.StatusBadRequest)
		return
	}

	if err := h.authStore.CreateUser(username); err != nil && !errors.Is(err, auth.ErrUserExists) {
		log.Printf("passkey: create user on invite: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.authStore.SavePasskey(username, cred, ""); err != nil {
		log.Printf("passkey: save passkey on invite: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.authStore.ConsumeInviteToken(token); err != nil {
		log.Printf("passkey: consume invite token: %v", err)
	}

	if !h.issueSessionCookie(w, r, username) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": username})
}

// ── Login ──────────────────────────────────────────────────────────────────

// passkeyLoginBegin handles POST /auth/passkey/login/begin. Body: {username?}.
func (h *Handler) passkeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil || h.webAuthn == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	var body struct {
		Username string `json:"username"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	username := strings.TrimSpace(body.Username)

	// Generate a random session ID so the browser can echo it back.
	sidBytes := make([]byte, 16)
	if _, err := rand.Read(sidBytes); err != nil {
		log.Printf("passkey: generate session id: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sid := hex.EncodeToString(sidBytes)

	var assertion *protocol.CredentialAssertion
	var session *webauthn.SessionData
	var err error

	if username != "" {
		user, userErr := h.authStore.GetWebAuthnUser(username)
		if userErr == nil {
			assertion, session, err = h.webAuthn.BeginLogin(user)
		}
	}
	if session == nil {
		assertion, session, err = h.webAuthn.BeginDiscoverableLogin()
	}
	if err != nil {
		log.Printf("passkey: begin login: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.challenges.set("login:"+sid, session)

	// Set a short-lived cookie so the finish step can find the session.
	http.SetCookie(w, &http.Cookie{
		Name:     passkeySessionCookie,
		Value:    sid,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   300,
	})

	writeJSON(w, http.StatusOK, assertion)
}

// passkeyLoginFinish handles POST /auth/passkey/login/finish.
func (h *Handler) passkeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil || h.webAuthn == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	sidCookie, err := r.Cookie(passkeySessionCookie)
	if err != nil {
		http.Error(w, "missing session cookie", http.StatusBadRequest)
		return
	}
	sid := strings.TrimSpace(sidCookie.Value)

	session, ok := h.challenges.get("login:" + sid)
	if !ok {
		http.Error(w, "no pending login session", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(r.Body)
	if err != nil {
		log.Printf("passkey: parse login credential: %v", err)
		http.Error(w, "invalid credential response", http.StatusBadRequest)
		return
	}

	h.challenges.delete("login:" + sid)

	var authenticatedUsername string
	var cred *webauthn.Credential

	// Try discoverable login first (authenticator provides userHandle = SHA256(username)).
	cred, err = h.webAuthn.ValidateDiscoverableLogin(
		func(rawID []byte, userHandle []byte) (webauthn.User, error) {
			users, listErr := h.authStore.ListUsers()
			if listErr != nil {
				return nil, listErr
			}
			hexID := hex.EncodeToString(rawID)
			for _, u := range users {
				waUser, waErr := h.authStore.GetWebAuthnUser(u.Username)
				if waErr != nil {
					continue
				}
				for _, c := range waUser.WebAuthnCredentials() {
					if hex.EncodeToString(c.ID) == hexID {
						authenticatedUsername = u.Username
						return waUser, nil
					}
				}
			}
			return nil, errors.New("credential not found")
		},
		*session,
		parsedResponse,
	)

	if err != nil || authenticatedUsername == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Clear the session cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     passkeySessionCookie,
		Value:    "",
		Path:     "/auth",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	if err := h.authStore.UpdatePasskeySignCount(cred.ID, cred.Authenticator.SignCount); err != nil {
		log.Printf("passkey: update sign count: %v", err)
	}

	if !h.issueSessionCookie(w, r, authenticatedUsername) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": authenticatedUsername})
}

// ── Invite management (protected) ─────────────────────────────────────────

// createInvite handles POST /api/invites.
func (h *Handler) createInvite(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Printf("passkey: generate invite token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(tokenBytes)
	expiresAt := time.Now().Add(30 * time.Minute)

	if err := h.authStore.CreateInviteToken(token, expiresAt); err != nil {
		log.Printf("passkey: store invite token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	inviteURL := scheme + "://" + r.Host + "/join?token=" + token

	writeJSON(w, http.StatusCreated, map[string]any{
		"url":       inviteURL,
		"token":     token,
		"expiresAt": expiresAt.UTC().Format(time.RFC3339),
	})
}

// ── Passkey management (protected) ────────────────────────────────────────

// listPasskeys handles GET /api/passkeys — scoped to the current user.
func (h *Handler) listPasskeys(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	claims, err := tokenFromRequest(r, h.jwtSecret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	passkeys, err := h.authStore.ListPasskeys(claims.Username)
	if err != nil {
		log.Printf("passkey: list: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type passkeyResponse struct {
		ID         string `json:"id"`
		DeviceName string `json:"deviceName"`
		CreatedAt  string `json:"createdAt"`
	}

	out := make([]passkeyResponse, 0, len(passkeys))
	for _, p := range passkeys {
		out = append(out, passkeyResponse{
			ID:         p.ID,
			DeviceName: p.DeviceName,
			CreatedAt:  p.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"passkeys": out})
}

// deletePasskey handles DELETE /api/passkeys/{id} — scoped to the current user.
func (h *Handler) deletePasskey(w http.ResponseWriter, r *http.Request) {
	if h.authStore == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return
	}

	claims, err := tokenFromRequest(r, h.jwtSecret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	credID := strings.TrimSpace(r.PathValue("id"))
	if credID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	if err := h.authStore.DeletePasskey(credID, claims.Username); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "passkey not found", http.StatusNotFound)
			return
		}
		log.Printf("passkey: delete: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── helpers ────────────────────────────────────────────────────────────────

func (h *Handler) setupIsAllowed(w http.ResponseWriter) bool {
	if h.authStore == nil || h.webAuthn == nil {
		http.Error(w, "auth not configured", http.StatusServiceUnavailable)
		return false
	}
	has, err := h.authStore.HasAnyUser()
	if err != nil {
		log.Printf("passkey: check users for setup: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return false
	}
	if has {
		http.Error(w, "setup already complete", http.StatusForbidden)
		return false
	}
	return true
}
