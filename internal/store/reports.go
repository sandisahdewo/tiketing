package store

import (
	"tiket/internal/model"
)

var periodExpr = map[string]func(date string) string{
	"daily":   func(date string) string { return "date(" + date + ")" },
	"weekly":  func(date string) string { return "date(" + date + ", 'weekday 0', '-6 days')" },
	"monthly": func(date string) string { return "strftime('%Y-%m', " + date + ")" },
}

// saleDates is the WHERE condition for a date range: the date of the sale, or
// with a shift type the date of its shift, so that a night shift past midnight
// stays whole.
func saleDates(from, to string, shiftTypeID int64) (string, []any, error) {
	start, end, err := dateRange(from, to)
	if err != nil {
		return "", nil, err
	}
	if shiftTypeID > 0 {
		return "s.shift_id IN (SELECT id FROM shifts WHERE type_id = ? AND business_date >= ? AND business_date <= ?)",
			[]any{shiftTypeID, from, to}, nil
	}
	return "s.created_at >= ? AND s.created_at < ?", []any{start, end}, nil
}

// Report summarises non-deleted sales. A userID > 0 limits it to that
// cashier and overrides q.UserID.
func (s *Store) Report(q model.ReportQuery, userID int64) (model.Report, error) {
	period, ok := periodExpr[q.Period]
	if !ok {
		return model.Report{}, model.Invalid("Periode laporan tidak valid")
	}
	dates, args, err := saleDates(q.From, q.To, q.ShiftTypeID)
	if err != nil {
		return model.Report{}, err
	}
	expr := period("s.created_at")
	if q.ShiftTypeID > 0 {
		expr = period("(SELECT business_date FROM shifts WHERE id = s.shift_id)")
	}
	report := model.Report{
		Period:  q.Period,
		From:    q.From,
		To:      q.To,
		Rows:    []model.ReportRow{},
		Tickets: []model.ReportTicketRow{},
	}
	where := dates + " AND s.deleted_at IS NULL"
	if userID == 0 {
		userID = q.UserID
	}
	if userID > 0 {
		where += " AND s.user_id = ?"
		args = append(args, userID)
	}
	if q.CounterID > 0 {
		where += " AND s.counter_id = ?"
		args = append(args, q.CounterID)
	}
	// Only the items of the category count; both queries join tickets t.
	if q.Category != "" {
		where += " AND t.category = ?"
		args = append(args, q.Category)
	}

	rows, err := s.DB.Query(
		`SELECT `+expr+` AS period, COUNT(DISTINCT s.id), SUM(i.qty), SUM(i.subtotal),
		        TOTAL(CASE WHEN t.category = 'ticket' THEN i.qty END),
		        TOTAL(CASE WHEN t.category = 'ticket' THEN i.subtotal END),
		        TOTAL(CASE WHEN t.category = 'topup' THEN i.qty END),
		        TOTAL(CASE WHEN t.category = 'topup' THEN i.subtotal END)
		 FROM sales s
		 JOIN sale_items i ON i.sale_id = s.id
		 JOIN tickets t ON t.id = i.ticket_id
		 WHERE `+where+`
		 GROUP BY period ORDER BY period`,
		args...,
	)
	if err != nil {
		return report, err
	}
	defer rows.Close()
	for rows.Next() {
		var r model.ReportRow
		var ticketQty, ticketTotal, topupQty, topupTotal float64
		if err := rows.Scan(&r.Period, &r.Transactions, &r.Qty, &r.Total, &ticketQty, &ticketTotal, &topupQty, &topupTotal); err != nil {
			return report, err
		}
		r.TicketQty, r.TicketTotal = int64(ticketQty), int64(ticketTotal)
		r.TopupQty, r.TopupTotal = int64(topupQty), int64(topupTotal)
		report.Rows = append(report.Rows, r)
		report.Transactions += r.Transactions
		report.Qty += r.Qty
		report.Total += r.Total
		report.TicketQty += r.TicketQty
		report.TicketTotal += r.TicketTotal
		report.TopupQty += r.TopupQty
		report.TopupTotal += r.TopupTotal
	}
	if err := rows.Err(); err != nil {
		return report, err
	}

	trows, err := s.DB.Query(
		`SELECT t.id, t.name, t.category, SUM(i.qty), SUM(i.subtotal)
		 FROM sales s
		 JOIN sale_items i ON i.sale_id = s.id
		 JOIN tickets t ON t.id = i.ticket_id
		 WHERE `+where+`
		 GROUP BY t.id, t.name, t.category ORDER BY t.category, SUM(i.subtotal) DESC, t.name`,
		args...,
	)
	if err != nil {
		return report, err
	}
	defer trows.Close()
	for trows.Next() {
		var r model.ReportTicketRow
		if err := trows.Scan(&r.TicketID, &r.TicketName, &r.Category, &r.Qty, &r.Total); err != nil {
			return report, err
		}
		report.Tickets = append(report.Tickets, r)
	}
	return report, trows.Err()
}

// ReportFilters lists every cashier (deleted ones last), every counter and
// every shift type.
func (s *Store) ReportFilters() (model.ReportFilters, error) {
	out := model.ReportFilters{Cashiers: []model.ReportCashier{}}
	rows, err := s.DB.Query(`SELECT id, full_name, deleted_at IS NOT NULL FROM users ORDER BY deleted_at IS NOT NULL, full_name COLLATE NOCASE`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var c model.ReportCashier
		if err := rows.Scan(&c.ID, &c.FullName, &c.Deleted); err != nil {
			return out, err
		}
		out.Cashiers = append(out.Cashiers, c)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if out.Counters, err = s.ListCounters(); err != nil {
		return out, err
	}
	out.ShiftTypes, err = s.ListShiftTypes(false)
	return out, err
}
