package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"

	"github.com/ercadev/lotsen/store"
	"golang.org/x/crypto/bcrypt"
)

// ValidBasicAuth compares HTTP Basic Auth credentials in constant time.
func ValidBasicAuth(r *http.Request, username, password string) bool {
	providedUser, providedPass, ok := r.BasicAuth()
	if !ok {
		return false
	}

	providedUserHash := sha256.Sum256([]byte(providedUser))
	expectedUserHash := sha256.Sum256([]byte(username))
	providedPassHash := sha256.Sum256([]byte(providedPass))
	expectedPassHash := sha256.Sum256([]byte(password))

	userMatch := subtle.ConstantTimeCompare(providedUserHash[:], expectedUserHash[:])
	passMatch := subtle.ConstantTimeCompare(providedPassHash[:], expectedPassHash[:])

	return userMatch == 1 && passMatch == 1
}

// ValidBasicAuthUsers checks request credentials against configured user hashes.
func ValidBasicAuthUsers(r *http.Request, auth *store.BasicAuthConfig) bool {
	if auth == nil || len(auth.Users) == 0 {
		return true
	}
	providedUser, providedPass, ok := r.BasicAuth()
	if !ok {
		return false
	}

	providedUserHash := sha256.Sum256([]byte(providedUser))
	for _, user := range auth.Users {
		expectedUserHash := sha256.Sum256([]byte(user.Username))
		if subtle.ConstantTimeCompare(providedUserHash[:], expectedUserHash[:]) != 1 {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(providedPass)) == nil {
			return true
		}
	}
	return false
}

// WriteBasicAuthChallenge writes a 401 challenge for Basic Auth.
func WriteBasicAuthChallenge(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", realm))
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
