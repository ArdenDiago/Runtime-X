package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidCreds is returned when username or password does not match.
	ErrInvalidCreds = errors.New("invalid username or password")

	// Store sessions in memory map[token]username
	sessions = struct {
		sync.RWMutex
		m map[string]string
	}{m: make(map[string]string)}

	// Pre-computed hash for password "Admin@1234"
	// Cost is bcrypt.DefaultCost
	// Generated using bcrypt.GenerateFromPassword([]byte("Admin@1234"), bcrypt.DefaultCost)
	adminHash = []byte("$2a$10$A2ISWIJ9oU1rkYNVfWRFtuGchsKiyIZUWcCYL8FDw9SG.YetCsjQC")
	
	// Single hardcoded user per assignment requirement
	adminUsername = "admin"
)

// Login verifies the credentials and returns a secure session token if valid.
func Login(username, password string) (string, error) {
	if username != adminUsername {
		return "", ErrInvalidCreds
	}

	err := bcrypt.CompareHashAndPassword(adminHash, []byte(password))
	if err != nil {
		return "", ErrInvalidCreds
	}

	// Generate a 32-byte secure token for the session
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	sessions.Lock()
	sessions.m[token] = username
	sessions.Unlock()

	return token, nil
}

// ValidateToken checks if a token exists in the session store.
func ValidateToken(token string) bool {
	sessions.RLock()
	_, exists := sessions.m[token]
	sessions.RUnlock()
	return exists
}

// Logout removes a token from the session store.
func Logout(token string) {
	sessions.Lock()
	delete(sessions.m, token)
	sessions.Unlock()
}
