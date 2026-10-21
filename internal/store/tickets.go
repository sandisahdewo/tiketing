package store

import (
	"database/sql"
	"strings"

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

// ticketSort maps the sortable columns of the ticket master list.
var ticketSort = map[string]string{
	"name":     "name COLLATE NOCASE",
	"category": "category",
	"price":    "price",
	"active":   "active",
	"created":  "created_at",
	"updated":  "updated_at",
	"deleted":  "deleted_at",
}

// SearchTickets returns one page of tickets matching q (already validated),
// by default ordered as on the cashier screen: tickets before topups, then by name.
func (s *Store) SearchTickets(q model.TicketQuery) (model.TicketPage, error) {
	out := model.TicketPage{Tickets: []model.Ticket{}, Page: q.Page, PageSize: q.PageSize}
	where := " WHERE deleted_at IS NULL"
	switch q.Status {
	case model.TicketDeleted:
		where = " WHERE deleted_at IS NOT NULL"
	case model.TicketActive:
		where += " AND active = 1"
	case model.TicketInactive:
		where += " AND active = 0"
	}
	var args []any
	if q.Category != "" {
		where += " AND category = ?"
		args = append(args, q.Category)
	}
	if search := strings.TrimSpace(q.Search); search != "" {
		where += ` AND name LIKE ? ESCAPE '\'`
		args = append(args, likePattern(search))
	}
	order, err := orderBy(ticketSort, q.Sort, q.Dir, "category, name, id", "id")
	if err != nil {
		return out, err
	}
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM tickets"+where, args...).Scan(&out.Count); err != nil || out.Count == 0 {
		return out, err
	}
	rows, err := s.DB.Query("SELECT "+ticketColumns+" FROM tickets"+where+order+" LIMIT ? OFFSET ?",
		append(args, q.PageSize, (q.Page-1)*q.PageSize)...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return out, err
		}
		out.Tickets = append(out.Tickets, t)
	}
	return out, rows.Err()
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
