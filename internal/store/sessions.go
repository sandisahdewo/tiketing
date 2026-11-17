package store

import "time"

// Sessions are the logins that are still valid, kept by the hash of their
// token (see auth.Sessions).

func (s *Store) CreateSession(tokenHash string, userID int64, expires time.Time) error {
	_, err := s.DB.Exec("INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)", tokenHash, userID, expires.Unix())
	return err
}

// RenewSession moves the end of a session. It does nothing for a session
// that was deleted meanwhile (logout, password changed), so that one is not
// brought back.
func (s *Store) RenewSession(tokenHash string, expires time.Time) error {
	_, err := s.DB.Exec("UPDATE sessions SET expires_at = ? WHERE token_hash = ?", expires.Unix(), tokenHash)
	return err
}

func (s *Store) Session(tokenHash string) (userID int64, expires time.Time, err error) {
	var unix int64
	err = s.DB.QueryRow("SELECT user_id, expires_at FROM sessions WHERE token_hash = ?", tokenHash).Scan(&userID, &unix)
	return userID, time.Unix(unix, 0), notFound(err)
}

func (s *Store) DeleteSession(tokenHash string) error {
	_, err := s.DB.Exec("DELETE FROM sessions WHERE token_hash = ?", tokenHash)
	return err
}

func (s *Store) DeleteUserSessions(userID int64) error {
	_, err := s.DB.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

func (s *Store) DeleteExpiredSessions(now time.Time) error {
	_, err := s.DB.Exec("DELETE FROM sessions WHERE expires_at < ?", now.Unix())
	return err
}

func (s *Store) DeleteAllSessions() error {
	_, err := s.DB.Exec("DELETE FROM sessions")
	return err
}
