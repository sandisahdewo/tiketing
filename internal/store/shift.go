package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"tiket/internal/model"
)

// dbtx is the database or a transaction on it. The database has a single
// connection, so while a transaction is open every query must go through it:
// one on s.DB would wait for that connection forever.
type dbtx interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

const clockLayout = "15:04"

// ShiftWindow places a shift with the hours start-end (HH:MM) opened at now:
// the business date it belongs to and when it is scheduled to start and end.
// An end at or before the start ends the next day, and such a shift opened
// after midnight but before its end belongs to the day before.
func ShiftWindow(start, end string, now time.Time) (date string, startAt, endAt time.Time, err error) {
	s, err := time.Parse(clockLayout, start)
	if err != nil {
		return "", startAt, endAt, model.Invalid("Jam mulai shift tidak valid")
	}
	e, err := time.Parse(clockLayout, end)
	if err != nil {
		return "", startAt, endAt, model.Invalid("Jam selesai shift tidak valid")
	}
	startMin, endMin := s.Hour()*60+s.Minute(), e.Hour()*60+e.Minute()
	if startMin == endMin {
		return "", startAt, endAt, model.Invalid("Jam selesai shift tidak boleh sama dengan jam mulai")
	}
	overnight := endMin < startMin
	day := now.Day()
	if overnight && now.Hour()*60+now.Minute() < endMin {
		day--
	}
	startAt = time.Date(now.Year(), now.Month(), day, s.Hour(), s.Minute(), 0, 0, now.Location())
	endDay := day
	if overnight {
		endDay++
	}
	endAt = time.Date(now.Year(), now.Month(), endDay, e.Hour(), e.Minute(), 0, 0, now.Location())
	return startAt.Format("2006-01-02"), startAt, endAt, nil
}

// SuggestShiftType picks the type a shift opened at now most likely is: the
// one running now (the latest to have started), else the next to start. It
// is 0 when there are no types.
func SuggestShiftType(types []model.ShiftType, now time.Time) int64 {
	var running, next int64
	var runningStart, nextStart time.Time
	for _, t := range types {
		_, startAt, endAt, err := ShiftWindow(t.StartTime, t.EndTime, now)
		if err != nil {
			continue
		}
		if !now.Before(startAt) && now.Before(endAt) {
			if running == 0 || startAt.After(runningStart) {
				running, runningStart = t.ID, startAt
			}
			continue
		}
		if !startAt.After(now) {
			startAt = startAt.AddDate(0, 0, 1)
		}
		if next == 0 || startAt.Before(nextStart) {
			next, nextStart = t.ID, startAt
		}
	}
	if running != 0 {
		return running
	}
	return next
}

const shiftTypeColumns = "id, name, start_time, end_time, active, created_at, updated_at"

func scanShiftType(row interface{ Scan(...any) error }) (model.ShiftType, error) {
	var t model.ShiftType
	err := row.Scan(&t.ID, &t.Name, &t.StartTime, &t.EndTime, &t.Active, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// ListShiftTypes lists the shift types by start time, turned-off ones last
// (or left out with activeOnly).
func (s *Store) ListShiftTypes(activeOnly bool) ([]model.ShiftType, error) {
	q := "SELECT " + shiftTypeColumns + " FROM shift_types"
	if activeOnly {
		q += " WHERE active = 1"
	}
	rows, err := s.DB.Query(q + " ORDER BY active DESC, start_time, name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ShiftType{}
	for rows.Next() {
		t, err := scanShiftType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetShiftType(id int64) (model.ShiftType, error) {
	t, err := scanShiftType(s.DB.QueryRow("SELECT "+shiftTypeColumns+" FROM shift_types WHERE id = ?", id))
	return t, notFound(err)
}

func shiftTypeConflict(err error) error {
	if isUniqueViolation(err) {
		return model.Invalid("Nama shift sudah dipakai")
	}
	return err
}

func (s *Store) CreateShiftType(in model.ShiftTypeInput) (model.ShiftType, error) {
	now := s.now()
	res, err := s.DB.Exec(
		"INSERT INTO shift_types (name, start_time, end_time, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		in.Name, in.StartTime, in.EndTime, now, now,
	)
	if err != nil {
		return model.ShiftType{}, shiftTypeConflict(err)
	}
	id, _ := res.LastInsertId()
	return s.GetShiftType(id)
}

// UpdateShiftType changes a type; shifts already opened keep the name and
// hours they were opened with.
func (s *Store) UpdateShiftType(in model.ShiftTypeInput) (model.ShiftType, error) {
	res, err := s.DB.Exec(
		"UPDATE shift_types SET name = ?, start_time = ?, end_time = ?, updated_at = ? WHERE id = ?",
		in.Name, in.StartTime, in.EndTime, s.now(), in.ID,
	)
	if err := affected(res, shiftTypeConflict(err)); err != nil {
		return model.ShiftType{}, err
	}
	return s.GetShiftType(in.ID)
}

// SetShiftTypeActive turns a type on or off; a turned-off type can no longer
// be opened, and shifts already open with it go on.
func (s *Store) SetShiftTypeActive(id int64, active bool) error {
	res, err := s.DB.Exec("UPDATE shift_types SET active = ?, updated_at = ? WHERE id = ?", active, s.now(), id)
	return affected(res, err)
}

// Shift queries keep the names of deleted users and show a counter by its
// name now, like the sale lists.
const shiftColumns = `h.id, h.type_id, h.name, h.start_time, h.end_time, h.business_date, h.scheduled_end_at,
	h.user_id, u.full_name, h.counter_id, c.name, h.opening_cash, h.opened_at, h.closed_at`

const shiftFrom = " FROM shifts h JOIN users u ON u.id = h.user_id JOIN counters c ON c.id = h.counter_id"

func scanShift(row interface{ Scan(...any) error }) (model.Shift, error) {
	var h model.Shift
	var closed sql.NullString
	err := row.Scan(&h.ID, &h.TypeID, &h.Name, &h.StartTime, &h.EndTime, &h.BusinessDate, &h.ScheduledEndAt,
		&h.UserID, &h.CashierName, &h.CounterID, &h.CounterName, &h.OpeningCash, &h.OpenedAt, &closed)
	if closed.Valid {
		h.ClosedAt = &closed.String
	}
	return h, err
}

func queryShifts(q dbtx, where string, args ...any) ([]model.Shift, error) {
	rows, err := q.Query("SELECT "+shiftColumns+shiftFrom+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Shift{}
	for rows.Next() {
		h, err := scanShift(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func getShift(q dbtx, id int64) (model.Shift, error) {
	h, err := scanShift(q.QueryRow("SELECT "+shiftColumns+shiftFrom+" WHERE h.id = ?", id))
	return h, notFound(err)
}

func (s *Store) GetShift(id int64) (model.Shift, error) {
	return getShift(s.DB, id)
}

// openShift is the open shift matching cond (on h), nil when there is none.
func openShift(q dbtx, cond string, arg any) (*model.Shift, error) {
	h, err := scanShift(q.QueryRow("SELECT "+shiftColumns+shiftFrom+" WHERE h.closed_at IS NULL AND "+cond, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// OpenShiftOf is the user's open shift, nil when there is none.
func (s *Store) OpenShiftOf(userID int64) (*model.Shift, error) {
	return openShift(s.DB, "h.user_id = ?", userID)
}

// OpenShiftAt is the open shift at a counter, nil when there is none.
func (s *Store) OpenShiftAt(counterID int64) (*model.Shift, error) {
	return openShift(s.DB, "h.counter_id = ?", counterID)
}

// deviceCounter is the active counter of the PC deviceID, for selling or
// opening a shift there.
func deviceCounter(q dbtx, deviceID string) (id int64, name, invoiceCode string, err error) {
	var active bool
	err = q.QueryRow("SELECT id, name, invoice_code, active FROM counters WHERE device_id = ?", deviceID).Scan(&id, &name, &invoiceCode, &active)
	if errors.Is(err, sql.ErrNoRows) || deviceID == "" {
		return 0, "", "", model.Invalid("Komputer ini belum punya loket. Minta admin memilih loket di Pengaturan.")
	}
	if err != nil {
		return 0, "", "", err
	}
	if !active {
		return 0, "", "", model.Invalid("Loket " + name + " sedang non-aktif")
	}
	return id, name, invoiceCode, nil
}

// OpenShift opens a shift of the type typeID for the user at the counter of
// the PC deviceID, with openingCash in the drawer.
func (s *Store) OpenShift(userID int64, deviceID string, typeID, openingCash int64) (model.Shift, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return model.Shift{}, err
	}
	defer tx.Rollback()
	counterID, counterName, _, err := deviceCounter(tx, deviceID)
	if err != nil {
		return model.Shift{}, err
	}
	t, err := scanShiftType(tx.QueryRow("SELECT "+shiftTypeColumns+" FROM shift_types WHERE id = ?", typeID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Shift{}, model.Invalid("Pilih shift yang dibuka")
	}
	if err != nil {
		return model.Shift{}, err
	}
	if !t.Active {
		return model.Shift{}, model.Invalid("Shift " + t.Name + " sedang non-aktif")
	}
	own, err := openShift(tx, "h.user_id = ?", userID)
	if err != nil {
		return model.Shift{}, err
	}
	if own != nil {
		return model.Shift{}, model.Invalid(fmt.Sprintf("Anda masih punya shift %s yang terbuka di %s. Tutup shift itu dulu.", own.Name, own.CounterName))
	}
	held, err := openShift(tx, "h.counter_id = ?", counterID)
	if err != nil {
		return model.Shift{}, err
	}
	if held != nil {
		return model.Shift{}, model.Invalid(fmt.Sprintf("%s masih dipakai shift %s milik %s. Shift itu harus ditutup dulu oleh %s atau admin.",
			counterName, held.Name, held.CashierName, held.CashierName))
	}
	now := s.Now()
	date, _, endAt, err := ShiftWindow(t.StartTime, t.EndTime, now)
	if err != nil {
		return model.Shift{}, err
	}
	res, err := tx.Exec(
		`INSERT INTO shifts (type_id, name, start_time, end_time, business_date, scheduled_end_at, user_id, counter_id, opening_cash, opened_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.StartTime, t.EndTime, date, endAt.Format(timeLayout), userID, counterID, openingCash, now.Format(timeLayout),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.Shift{}, model.Invalid("Shift baru saja dibuka dari komputer lain. Muat ulang.")
		}
		return model.Shift{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Shift{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetShift(id)
}

func idArgs(ids []int64) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return "(?" + strings.Repeat(", ?", len(ids)-1) + ")", args
}

// shiftSales counts the sales of each of the shifts ids.
func shiftSales(q dbtx, ids []int64) (map[int64]model.ShiftSales, error) {
	out := make(map[int64]model.ShiftSales, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := idArgs(ids)
	rows, err := q.Query(
		`SELECT s.shift_id,
		        COUNT(CASE WHEN s.deleted_at IS NULL THEN 1 END),
		        COALESCE(SUM(CASE WHEN s.deleted_at IS NULL THEN s.total END), 0),
		        COALESCE(MIN(CASE WHEN s.deleted_at IS NULL THEN s.created_at END), ''),
		        COALESCE(MAX(CASE WHEN s.deleted_at IS NULL THEN s.created_at END), ''),
		        COUNT(s.deleted_at),
		        COALESCE(SUM(CASE WHEN s.deleted_at IS NOT NULL THEN s.total END), 0)
		 FROM sales s
		 WHERE s.shift_id IN `+in+`
		 GROUP BY s.shift_id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var d model.ShiftSales
		if err := rows.Scan(&id, &d.Transactions, &d.Total, &d.FirstSaleAt, &d.LastSaleAt, &d.DeletedCount, &d.DeletedTotal); err != nil {
			rows.Close()
			return nil, err
		}
		out[id] = d
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items, err := q.Query(
		`SELECT s.shift_id, t.category, COALESCE(SUM(i.qty), 0), COALESCE(SUM(i.subtotal), 0)
		 FROM sales s
		 JOIN sale_items i ON i.sale_id = s.id
		 JOIN tickets t ON t.id = i.ticket_id
		 WHERE s.shift_id IN `+in+` AND s.deleted_at IS NULL
		 GROUP BY s.shift_id, t.category`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer items.Close()
	for items.Next() {
		var id, qty, total int64
		var category string
		if err := items.Scan(&id, &category, &qty, &total); err != nil {
			return nil, err
		}
		d := out[id]
		switch category {
		case model.CategoryTicket:
			d.TicketQty, d.TicketTotal = qty, total
		case model.CategoryTopup:
			d.TopupQty, d.TopupTotal = qty, total
		}
		out[id] = d
	}
	return out, items.Err()
}

const closingColumns = `k.id, k.shift_id, k.revision, k.transactions, k.ticket_qty, k.ticket_total, k.topup_qty,
	k.topup_total, k.total, COALESCE(k.first_sale_at, ''), COALESCE(k.last_sale_at, ''), k.deleted_count,
	k.deleted_total, k.actual_cash, k.note, b.full_name, k.closed_at, k.print_count`

const closingFrom = " FROM shift_closings k JOIN users b ON b.id = k.closed_by"

// scanClosing reads a closing; withShift completes it.
func scanClosing(row interface{ Scan(...any) error }) (model.ShiftClosing, error) {
	var c model.ShiftClosing
	d := &c.Sales
	err := row.Scan(&c.ID, &c.Shift.ID, &c.Revision, &d.Transactions, &d.TicketQty, &d.TicketTotal, &d.TopupQty,
		&d.TopupTotal, &d.Total, &d.FirstSaleAt, &d.LastSaleAt, &d.DeletedCount, &d.DeletedTotal, &c.ActualCash,
		&c.Note, &c.ClosedByName, &c.ClosedAt, &c.PrintCount)
	return c, err
}

// withShift adds the shift to a closing and what it counts from it.
func withShift(c *model.ShiftClosing, h model.Shift) {
	c.Shift = h
	c.ExpectedCash = h.OpeningCash + c.Sales.Total
	c.Difference = c.ActualCash - c.ExpectedCash
}

// closingsOf lists the closings of the shifts, each shift's newest revision
// first, with PreviousActual set on corrections.
func closingsOf(q dbtx, shifts map[int64]model.Shift, ids []int64) (map[int64][]model.ShiftClosing, error) {
	out := make(map[int64][]model.ShiftClosing, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := idArgs(ids)
	rows, err := q.Query("SELECT "+closingColumns+closingFrom+" WHERE k.shift_id IN "+in+" ORDER BY k.shift_id, k.revision DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanClosing(rows)
		if err != nil {
			return nil, err
		}
		withShift(&c, shifts[c.Shift.ID])
		out[c.Shift.ID] = append(out[c.Shift.ID], c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, list := range out {
		for i := 0; i+1 < len(list); i++ {
			prev := list[i+1].ActualCash
			list[i].PreviousActual = &prev
		}
	}
	return out, nil
}

// deletedSales lists the deleted sales of each of the shifts, in the order
// they were deleted.
func deletedSales(q dbtx, ids []int64) (map[int64][]model.DeletedSale, error) {
	out := make(map[int64][]model.DeletedSale, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	in, args := idArgs(ids)
	rows, err := q.Query(
		`SELECT s.shift_id, s.id, s.invoice_no, s.total, s.created_at, s.deleted_at, COALESCE(d.full_name, ''), s.delete_reason,
		        COALESCE((SELECT n.invoice_no FROM sales n WHERE n.replaces_sale_id = s.id), '')
		 FROM sales s LEFT JOIN users d ON d.id = s.deleted_by
		 WHERE s.shift_id IN `+in+` AND s.deleted_at IS NOT NULL
		 ORDER BY s.deleted_at, s.id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var shiftID int64
		var d model.DeletedSale
		if err := rows.Scan(&shiftID, &d.ID, &d.InvoiceNo, &d.Total, &d.CreatedAt, &d.DeletedAt, &d.DeletedByName, &d.Reason, &d.ReplacedByInvoiceNo); err != nil {
			return nil, err
		}
		out[shiftID] = append(out[shiftID], d)
	}
	return out, rows.Err()
}

// firstDeleted is the first n deleted sales: those of a closing that counted
// n. A sale stays deleted, so the ones deleted first are the ones it counted.
func firstDeleted(list []model.DeletedSale, n int64) []model.DeletedSale {
	return append([]model.DeletedSale{}, list[:min(int(n), len(list))]...)
}

// reports adds to each shift its sales now and its closings.
func reports(q dbtx, shifts []model.Shift) ([]model.ShiftReport, error) {
	ids := make([]int64, len(shifts))
	byID := make(map[int64]model.Shift, len(shifts))
	for i, h := range shifts {
		ids[i] = h.ID
		byID[h.ID] = h
	}
	sales, err := shiftSales(q, ids)
	if err != nil {
		return nil, err
	}
	closings, err := closingsOf(q, byID, ids)
	if err != nil {
		return nil, err
	}
	deleted, err := deletedSales(q, ids)
	if err != nil {
		return nil, err
	}
	out := make([]model.ShiftReport, 0, len(shifts))
	for _, h := range shifts {
		r := model.ShiftReport{Shift: h, Sales: sales[h.ID], History: closings[h.ID], Deleted: deleted[h.ID]}
		if r.History == nil {
			r.History = []model.ShiftClosing{}
		}
		if r.Deleted == nil {
			r.Deleted = []model.DeletedSale{}
		}
		for i := range r.History {
			r.History[i].Deleted = firstDeleted(r.Deleted, r.History[i].Sales.DeletedCount)
		}
		r.ExpectedCash = h.OpeningCash + r.Sales.Total
		if len(r.History) > 0 {
			latest := r.History[0]
			r.Closing = &latest
			r.Changed = latest.Sales.Total != r.Sales.Total ||
				latest.Sales.Transactions != r.Sales.Transactions ||
				latest.Sales.DeletedCount != r.Sales.DeletedCount
		}
		out = append(out, r)
	}
	return out, nil
}

// ShiftReport is a shift with its sales now and its closings.
func (s *Store) ShiftReport(id int64) (model.ShiftReport, error) {
	h, err := s.GetShift(id)
	if err != nil {
		return model.ShiftReport{}, err
	}
	list, err := reports(s.DB, []model.Shift{h})
	if err != nil {
		return model.ShiftReport{}, err
	}
	return list[0], nil
}

// ShiftsOfDay lists the shifts of a business date in the order they opened.
func (s *Store) ShiftsOfDay(date string) ([]model.ShiftReport, error) {
	shifts, err := queryShifts(s.DB, " WHERE h.business_date = ? ORDER BY h.opened_at, h.id", date)
	if err != nil {
		return nil, err
	}
	return reports(s.DB, shifts)
}

// UnclosedShifts lists the shifts still open of every business date but one.
func (s *Store) UnclosedShifts(exceptDate string) ([]model.Shift, error) {
	return queryShifts(s.DB, " WHERE h.closed_at IS NULL AND h.business_date <> ? ORDER BY h.opened_at, h.id", exceptDate)
}

// UserShifts lists a user's shifts that are open or were closed on day.
func (s *Store) UserShifts(userID int64, day string) ([]model.ShiftReport, error) {
	start, end, err := dateRange(day, day)
	if err != nil {
		return nil, err
	}
	shifts, err := queryShifts(s.DB,
		" WHERE h.user_id = ? AND (h.closed_at IS NULL OR (h.closed_at >= ? AND h.closed_at < ?)) ORDER BY h.opened_at, h.id",
		userID, start, end)
	if err != nil {
		return nil, err
	}
	return reports(s.DB, shifts)
}

// CloseShift records the cash counted at the end of a shift together with
// the shift's sales as they are now. Closing a closed shift again is a
// correction: the next revision, which needs a note saying why. A first
// closing needs a note too when the cash differs from what it should be.
// in.Revision must be the revision this closing becomes.
func (s *Store) CloseShift(in model.CloseShiftInput, closedBy int64) (model.ShiftClosing, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return model.ShiftClosing{}, err
	}
	defer tx.Rollback()
	h, err := getShift(tx, in.ShiftID)
	if err != nil {
		return model.ShiftClosing{}, err
	}
	var next int64
	if err := tx.QueryRow("SELECT COALESCE(MAX(revision) + 1, 0) FROM shift_closings WHERE shift_id = ?", h.ID).Scan(&next); err != nil {
		return model.ShiftClosing{}, err
	}
	if in.Revision != next {
		return model.ShiftClosing{}, model.Invalid("Shift ini baru saja ditutup atau dikoreksi dari komputer lain. Muat ulang lalu koreksi bila perlu.")
	}
	sales, err := shiftSales(tx, []int64{h.ID})
	if err != nil {
		return model.ShiftClosing{}, err
	}
	d := sales[h.ID]
	if next > 0 && in.Note == "" {
		return model.ShiftClosing{}, model.Invalid("Alasan koreksi wajib diisi")
	}
	if in.ActualCash != h.OpeningCash+d.Total && in.Note == "" {
		return model.ShiftClosing{}, model.Invalid("Keterangan selisih wajib diisi")
	}
	now := s.now()
	if next == 0 {
		if _, err := tx.Exec("UPDATE shifts SET closed_at = ? WHERE id = ?", now, h.ID); err != nil {
			return model.ShiftClosing{}, err
		}
	}
	nullable := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	res, err := tx.Exec(
		`INSERT INTO shift_closings (shift_id, revision, transactions, ticket_qty, ticket_total, topup_qty, topup_total,
		  total, first_sale_at, last_sale_at, deleted_count, deleted_total, actual_cash, note, closed_by, closed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, next, d.Transactions, d.TicketQty, d.TicketTotal, d.TopupQty, d.TopupTotal, d.Total,
		nullable(d.FirstSaleAt), nullable(d.LastSaleAt), d.DeletedCount, d.DeletedTotal, in.ActualCash, in.Note, closedBy, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ShiftClosing{}, model.Invalid("Shift ini baru saja ditutup atau dikoreksi dari komputer lain. Muat ulang lalu koreksi bila perlu.")
		}
		return model.ShiftClosing{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.ShiftClosing{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetShiftClosing(id)
}

func (s *Store) GetShiftClosing(id int64) (model.ShiftClosing, error) {
	c, err := scanClosing(s.DB.QueryRow("SELECT "+closingColumns+closingFrom+" WHERE k.id = ?", id))
	if err != nil {
		return c, notFound(err)
	}
	h, err := s.GetShift(c.Shift.ID)
	if err != nil {
		return c, err
	}
	withShift(&c, h)
	var prev int64
	err = s.DB.QueryRow("SELECT actual_cash FROM shift_closings WHERE shift_id = ? AND revision = ?", h.ID, c.Revision-1).Scan(&prev)
	switch {
	case err == nil:
		c.PreviousActual = &prev
	case !errors.Is(err, sql.ErrNoRows):
		return c, err
	}
	return c, nil
}

// RecordShiftClosingPrint counts a print of a closing, so the next one says
// it is a reprint.
func (s *Store) RecordShiftClosingPrint(id int64) error {
	res, err := s.DB.Exec("UPDATE shift_closings SET print_count = print_count + 1 WHERE id = ?", id)
	return affected(res, err)
}
