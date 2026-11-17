package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"tiket/internal/model"
)

const (
	// A session ends after this long without any use.
	sessionTTL = 24 * time.Hour
	// Using a session extends it, but that is written at most this often, so
	// a busy cashier does not cause a write with every request.
	renewEvery = 5 * time.Minute
)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// SessionStore keeps the sessions: the database of the server PC, so they
// are still there after the app is closed and opened again.
type SessionStore interface {
	CreateSession(tokenHash string, userID int64, expires time.Time) error
	// RenewSession does nothing for a session that no longer exists.
	RenewSession(tokenHash string, expires time.Time) error
	// Session returns model.ErrNotFound for a token that has no session.
	Session(tokenHash string) (userID int64, expires time.Time, err error)
	DeleteSession(tokenHash string) error
	DeleteUserSessions(userID int64) error
	DeleteExpiredSessions(now time.Time) error
}

type Sessions struct {
	store SessionStore
	now   func() time.Time
}

func NewSessions(store SessionStore, now func() time.Time) *Sessions {
	return &Sessions{store: store, now: now}
}

// tokenHash is what the store keeps instead of the token itself.
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Sessions) Create(userID int64) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	now := s.now()
	// A login is a good moment to drop the sessions nobody came back to.
	if err := s.store.DeleteExpiredSessions(now); err != nil {
		return "", err
	}
	if err := s.store.CreateSession(tokenHash(token), userID, now.Add(sessionTTL)); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Sessions) Lookup(token string) (int64, error) {
	if token == "" {
		return 0, model.ErrUnauthorized
	}
	hash := tokenHash(token)
	userID, expires, err := s.store.Session(hash)
	if errors.Is(err, model.ErrNotFound) {
		return 0, model.ErrUnauthorized
	}
	if err != nil {
		return 0, err
	}
	now := s.now()
	if now.After(expires) {
		// Tidying up; the session is over whether or not this works.
		s.store.DeleteSession(hash)
		return 0, model.ErrUnauthorized
	}
	if expires.Sub(now) < sessionTTL-renewEvery {
		// A renewal that fails is tried again with the next request; the
		// session is valid until then, so the request itself goes on.
		s.store.RenewSession(hash, now.Add(sessionTTL))
	}
	return userID, nil
}

func (s *Sessions) Delete(token string) error {
	return s.store.DeleteSession(tokenHash(token))
}

// DeleteUser logs the user out everywhere.
func (s *Sessions) DeleteUser(userID int64) error {
	return s.store.DeleteUserSessions(userID)
}
