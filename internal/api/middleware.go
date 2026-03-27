package api

import (
	"errors"
	"net/http"

	"runtimex/internal/auth"
)

// requireAuth intercepts the request and verifies the session cookie securely.
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("rtx_session")
		if err != nil {
			send(w, http.StatusUnauthorized, nil, errors.New("missing session cookie: please login"))
			return
		}

		if !auth.ValidateToken(cookie.Value) {
			send(w, http.StatusUnauthorized, nil, errors.New("invalid or expired session: please login again"))
			return
		}

		// Valid session: proceed to the next handler
		next.ServeHTTP(w, r)
	}
}
