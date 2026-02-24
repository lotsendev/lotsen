package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
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

// WriteBasicAuthChallenge writes a 401 challenge for Basic Auth.
func WriteBasicAuthChallenge(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Basic realm=%q", realm))
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
