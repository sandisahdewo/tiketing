package store

import (
	"database/sql"

	"tiket/internal/model"
)

const ticketColumns = "id, name, price, category, color, active, created_at, updated_at, deleted_at"

func scanTicket(row interface{ Scan(...any) error }) (model.Ticket, error) {
	var t model.Ticket
	var deleted sql.NullString
	err := row.Scan(&t.ID, &t.Name, &t.Price, &t.Category, &t.Color, &t.Active, &t.CreatedAt, &t.UpdatedAt, &deleted)
	if deleted.Valid {
		t.DeletedAt = &deleted.String
	}
	return t, err
}

// ListTickets lists deleted or non-deleted tickets; activeOnly hides the
// inactive ones (for the cashier screen).
func (s *Store) ListTickets(search string, deleted, activeOnly bool) ([]model.Ticket, error) {
	cond := "deleted_at IS NULL"
	if deleted {
		cond = "deleted_at IS NOT NULL"
	}
	if activeOnly {
		cond += " AND active = 1"
	}
	rows, err := s.DB.Query(
		"SELECT "+ticketColumns+" FROM tickets WHERE "+cond+` AND name LIKE ? ESCAPE '\' ORDER BY category, name`,
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

func (s *Store) CreateTicket(in model.TicketInput) (model.Ticket, error) {
	now := s.now()
	res, err := s.DB.Exec(
		"INSERT INTO tickets (name, price, category, color, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		in.Name, in.Price, in.Category, in.Color, now, now,
	)
	if err != nil {
		return model.Ticket{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetTicket(id)
}

func (s *Store) UpdateTicket(in model.TicketInput) (model.Ticket, error) {
	res, err := s.DB.Exec(
		"UPDATE tickets SET name = ?, price = ?, category = ?, color = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL",
		in.Name, in.Price, in.Category, in.Color, s.now(), in.ID,
	)
	if err := affected(res, err); err != nil {
		return model.Ticket{}, err
	}
	return s.GetTicket(in.ID)
}

func (s *Store) DeleteTicket(id int64) error {
	now := s.now()
	res, err := s.DB.Exec(
		"UPDATE tickets SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL",
		now, now, id,
	)
	return affected(res, err)
}

func (s *Store) SetTicketActive(id int64, active bool) error {
	res, err := s.DB.Exec(
		"UPDATE tickets SET active = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL",
		active, s.now(), id,
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
