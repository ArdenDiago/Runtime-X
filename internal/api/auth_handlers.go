package api

import (
	"encoding/json"
	"net/http"
	"time"

	"runtimex/internal/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var body loginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		send(w, http.StatusBadRequest, nil, err)
		return
	}

	token, err := auth.Login(body.Username, body.Password)
	if err != nil {
		send(w, http.StatusUnauthorized, nil, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "rtx_session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	send(w, http.StatusOK, map[string]string{"message": "login successful"}, nil)
}

func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("rtx_session")
	if err == nil {
		auth.Logout(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "rtx_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})

	send(w, http.StatusOK, map[string]string{"message": "logged out"}, nil)
}

func (s *Server) HandleCheckAuth(w http.ResponseWriter, r *http.Request) {
	// Middleware will intercept invalid sessions.
	// If we reach here, the session is active.
	send(w, http.StatusOK, map[string]bool{"authenticated": true}, nil)
}
