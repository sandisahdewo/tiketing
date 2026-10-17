package store

import (
	"tiket/internal/model"
)

var periodExpr = map[string]string{
	"daily":   "date(s.created_at)",
	"weekly":  "date(s.created_at, 'weekday 0', '-6 days')",
	"monthly": "strftime('%Y-%m', s.created_at)",
}

func (s *Store) Report(q model.ReportQuery) (model.Report, error) {
	expr, ok := periodExpr[q.Period]
	if !ok {
		return model.Report{}, model.Invalid("Periode laporan tidak valid")
	}
	start, end, err := dateRange(q.From, q.To)
	if err != nil {
		return model.Report{}, err
	}
	report := model.Report{
		Period:  q.Period,
		From:    q.From,
		To:      q.To,
		Rows:    []model.ReportRow{},
		Tickets: []model.ReportTicketRow{},
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
		 WHERE s.created_at >= ? AND s.created_at < ?
		 GROUP BY period ORDER BY period`,
		start, end,
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
		 WHERE s.created_at >= ? AND s.created_at < ?
		 GROUP BY t.id, t.name, t.category ORDER BY t.category, SUM(i.subtotal) DESC, t.name`,
		start, end,
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
