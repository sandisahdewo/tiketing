package store

import (
	"tiket/internal/model"
)

const userColumns = "id, username, full_name, role, created_at, updated_at"

func scanUser(row interface{ Scan(...any) error }) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (s *Store) CountUsers() (int64, error) {
	var n int64
	err := s.DB.QueryRow("SELECT COUNT(*) FROM users WHERE deleted_at IS NULL").Scan(&n)
	return n, err
}

func (s *Store) CountAdmins() (int64, error) {
	var n int64
	err := s.DB.QueryRow("SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND role = 'admin'").Scan(&n)
	return n, err
}

func (s *Store) ListUsers() ([]model.User, error) {
	rows, err := s.DB.Query("SELECT " + userColumns + " FROM users WHERE deleted_at IS NULL ORDER BY username")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []model.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) GetUser(id int64) (model.User, error) {
	u, err := scanUser(s.DB.QueryRow("SELECT "+userColumns+" FROM users WHERE id = ? AND deleted_at IS NULL", id))
	return u, notFound(err)
}

func (s *Store) GetUserForLogin(username string) (model.User, string, error) {
	var u model.User
	var hash string
	err := s.DB.QueryRow(
		"SELECT "+userColumns+", password_hash FROM users WHERE username = ? AND deleted_at IS NULL",
		username,
	).Scan(&u.ID, &u.Username, &u.FullName, &u.Role, &u.CreatedAt, &u.UpdatedAt, &hash)
	return u, hash, notFound(err)
}

func (s *Store) CreateUser(username, fullName, role, passwordHash string) (model.User, error) {
	now := s.now()
	res, err := s.DB.Exec(
		`INSERT INTO users (username, full_name, password_hash, role, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		username, fullName, passwordHash, role, now, now,
	)
	if isUniqueViolation(err) {
		return model.User{}, model.Invalid("Username sudah dipakai")
	}
	if err != nil {
		return model.User{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetUser(id)
}

func (s *Store) UpdateUser(id int64, username, fullName, role string) (model.User, error) {
	res, err := s.DB.Exec(
		"UPDATE users SET username = ?, full_name = ?, role = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL",
		username, fullName, role, s.now(), id,
	)
	if isUniqueViolation(err) {
		return model.User{}, model.Invalid("Username sudah dipakai")
	}
	if err := affected(res, err); err != nil {
		return model.User{}, err
	}
	return s.GetUser(id)
}

func (s *Store) SetPassword(id int64, passwordHash string) error {
	res, err := s.DB.Exec(
		"UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL",
		passwordHash, s.now(), id,
	)
	return affected(res, err)
}

func (s *Store) DeleteUser(id int64) error {
	now := s.now()
	res, err := s.DB.Exec(
		"UPDATE users SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL",
		now, now, id,
	)
	return affected(res, err)
}
