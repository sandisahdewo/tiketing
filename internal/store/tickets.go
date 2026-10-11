package store

import (
	"database/sql"

	"tiket/internal/model"
)

const ticketColumns = "id, name, price, created_at, updated_at, deleted_at"

func scanTicket(row interface{ Scan(...any) error }) (model.Ticket, error) {
	var t model.Ticket
	var deleted sql.NullString
	err := row.Scan(&t.ID, &t.Name, &t.Price, &t.CreatedAt, &t.UpdatedAt, &deleted)
	if deleted.Valid {
		t.DeletedAt = &deleted.String
	}
	return t, err
}

func (s *Store) ListTickets(search string, deleted bool) ([]model.Ticket, error) {
	cond := "deleted_at IS NULL"
	if deleted {
		cond = "deleted_at IS NOT NULL"
	}
	rows, err := s.DB.Query(
		"SELECT "+ticketColumns+" FROM tickets WHERE "+cond+` AND name LIKE ? ESCAPE '\' ORDER BY name`,
		likePattern(search),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tickets := []model.Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

func (s *Store) GetTicket(id int64) (model.Ticket, error) {
	t, err := scanTicket(s.DB.QueryRow("SELECT "+ticketColumns+" FROM tickets WHERE id = ?", id))
	return t, notFound(err)
}

func (s *Store) CreateTicket(name string, price int64) (model.Ticket, error) {
	now := s.now()
	res, err := s.DB.Exec(
		"INSERT INTO tickets (name, price, created_at, updated_at) VALUES (?, ?, ?, ?)",
		name, price, now, now,
	)
	if err != nil {
		return model.Ticket{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetTicket(id)
}

func (s *Store) UpdateTicket(id int64, name string, price int64) (model.Ticket, error) {
	res, err := s.DB.Exec(
		"UPDATE tickets SET name = ?, price = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL",
		name, price, s.now(), id,
	)
	if err := affected(res, err); err != nil {
		return model.Ticket{}, err
	}
	return s.GetTicket(id)
}

func (s *Store) DeleteTicket(id int64) error {
	now := s.now()
	res, err := s.DB.Exec(
		"UPDATE tickets SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL",
		now, now, id,
	)
	return affected(res, err)
}

func (s *Store) RestoreTicket(id int64) error {
	res, err := s.DB.Exec(
		"UPDATE tickets SET deleted_at = NULL, updated_at = ? WHERE id = ? AND deleted_at IS NOT NULL",
		s.now(), id,
	)
	return affected(res, err)
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}
