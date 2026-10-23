package store

import (
	"database/sql"
	"strings"

	"tiket/internal/model"
)

const counterColumns = "id, name, invoice_code, active, COALESCE(device_id, ''), device_name, last_seen_at, created_at, updated_at"

func scanCounter(row interface{ Scan(...any) error }) (model.Counter, error) {
	var c model.Counter
	var seen sql.NullString
	err := row.Scan(&c.ID, &c.Name, &c.InvoiceCode, &c.Active, &c.DeviceID, &c.DeviceName, &seen, &c.CreatedAt, &c.UpdatedAt)
	if seen.Valid {
		c.LastSeenAt = &seen.String
	}
	return c, err
}

// ListCounters lists all counters, active ones first, by name.
func (s *Store) ListCounters() ([]model.Counter, error) {
	rows, err := s.DB.Query("SELECT " + counterColumns + " FROM counters ORDER BY active DESC, name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Counter{}
	for rows.Next() {
		c, err := scanCounter(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCounter(id int64) (model.Counter, error) {
	c, err := scanCounter(s.DB.QueryRow("SELECT "+counterColumns+" FROM counters WHERE id = ?", id))
	return c, notFound(err)
}

// CounterByDevice is the counter of the PC deviceID, or ErrNotFound.
func (s *Store) CounterByDevice(deviceID string) (model.Counter, error) {
	if deviceID == "" {
		return model.Counter{}, model.ErrNotFound
	}
	c, err := scanCounter(s.DB.QueryRow("SELECT "+counterColumns+" FROM counters WHERE device_id = ?", deviceID))
	return c, notFound(err)
}

// counterConflict turns a unique-index violation into a readable message.
func counterConflict(err error) error {
	if !isUniqueViolation(err) {
		return err
	}
	if strings.Contains(err.Error(), "invoice_code") {
		return model.Invalid("Kode nota sudah dipakai loket lain")
	}
	return model.Invalid("Nama loket sudah dipakai")
}

func (s *Store) CreateCounter(in model.CounterInput) (model.Counter, error) {
	now := s.now()
	res, err := s.DB.Exec(
		"INSERT INTO counters (name, invoice_code, created_at, updated_at) VALUES (?, ?, ?, ?)",
		in.Name, in.InvoiceCode, now, now,
	)
	if err != nil {
		return model.Counter{}, counterConflict(err)
	}
	id, _ := res.LastInsertId()
	return s.GetCounter(id)
}

func (s *Store) UpdateCounter(in model.CounterInput) (model.Counter, error) {
	res, err := s.DB.Exec(
		"UPDATE counters SET name = ?, invoice_code = ?, updated_at = ? WHERE id = ?",
		in.Name, in.InvoiceCode, s.now(), in.ID,
	)
	if err := affected(res, counterConflict(err)); err != nil {
		return model.Counter{}, err
	}
	return s.GetCounter(in.ID)
}

// SetCounterActive turns a counter on or off. A counter that is turned off
// is also taken off its PC, which then cannot sell until it gets another one.
func (s *Store) SetCounterActive(id int64, active bool) error {
	q := "UPDATE counters SET active = ?, updated_at = ? WHERE id = ?"
	if !active {
		q = "UPDATE counters SET active = ?, device_id = NULL, device_name = '', updated_at = ? WHERE id = ?"
	}
	res, err := s.DB.Exec(q, active, s.now(), id)
	return affected(res, err)
}

// ErrCounterTaken means the counter is in use by another PC; the message
// names that PC.
type ErrCounterTaken struct{ Device string }

func (e *ErrCounterTaken) Error() string {
	return "Loket ini sedang dipakai komputer " + e.Device
}

// AssignCounter puts the PC deviceID on a counter, taking it off any counter
// it had. A counter used by another PC is only taken over with force, which
// leaves that PC without a counter. counterID 0 only frees the PC.
func (s *Store) AssignCounter(counterID int64, deviceID, deviceName string, force bool) error {
	if deviceID == "" {
		return model.Invalid("Komputer tidak dikenali")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	if counterID > 0 {
		var active bool
		var current, currentName sql.NullString
		err := tx.QueryRow("SELECT active, device_id, device_name FROM counters WHERE id = ?", counterID).Scan(&active, &current, &currentName)
		if err != nil {
			return notFound(err)
		}
		if !active {
			return model.Invalid("Loket ini non-aktif")
		}
		if current.Valid && current.String != deviceID && !force {
			name := currentName.String
			if name == "" {
				name = "lain"
			}
			return &ErrCounterTaken{Device: name}
		}
	}
	if _, err := tx.Exec("UPDATE counters SET device_id = NULL, device_name = '', updated_at = ? WHERE device_id = ? AND id <> ?", now, deviceID, counterID); err != nil {
		return err
	}
	if counterID > 0 {
		if _, err := tx.Exec("UPDATE counters SET device_id = ?, device_name = ?, last_seen_at = ?, updated_at = ? WHERE id = ?", deviceID, deviceName, now, now, counterID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// TouchCounter records that the PC deviceID was just used.
func (s *Store) TouchCounter(deviceID string) {
	if deviceID != "" {
		s.DB.Exec("UPDATE counters SET last_seen_at = ? WHERE device_id = ?", s.now(), deviceID)
	}
}
