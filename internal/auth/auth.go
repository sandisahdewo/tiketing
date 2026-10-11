package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"tiket/internal/model"
)

const sessionTTL = 24 * time.Hour

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

type session struct {
	userID  int64
	expires time.Time
}

type Sessions struct {
	mu   sync.Mutex
	data map[string]session
	now  func() time.Time
}

func NewSessions() *Sessions {
	return &Sessions{data: map[string]session{}, now: time.Now}
}

func (s *Sessions) Create(userID int64) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[token] = session{userID: userID, expires: s.now().Add(sessionTTL)}
	return token, nil
}

func (s *Sessions) Lookup(token string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.data[token]
	if !ok || s.now().After(sess.expires) {
		delete(s.data, token)
		return 0, model.ErrUnauthorized
	}
	sess.expires = s.now().Add(sessionTTL)
	s.data[token] = sess
	return sess.userID, nil
}

func (s *Sessions) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, token)
}

func (s *Sessions) DeleteUser(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, sess := range s.data {
		if sess.userID == userID {
			delete(s.data, token)
		}
	}
}
