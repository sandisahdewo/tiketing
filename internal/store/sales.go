package store

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"tiket/internal/model"
)

const saleColumns = `s.id, s.invoice_no, s.user_id, u.full_name, s.terminal, s.total, s.paid, s.change_amount, s.created_at,
	(SELECT COUNT(*) FROM sale_prints p WHERE p.sale_id = s.id), s.deleted_at, COALESCE(d.full_name, ''), s.delete_reason`

const saleFrom = " FROM sales s JOIN users u ON u.id = s.user_id LEFT JOIN users d ON d.id = s.deleted_by"

func scanSale(row interface{ Scan(...any) error }) (model.Sale, error) {
	var s model.Sale
	err := row.Scan(&s.ID, &s.InvoiceNo, &s.UserID, &s.CashierName, &s.Terminal, &s.Total, &s.Paid, &s.ChangeAmount, &s.CreatedAt, &s.PrintCount,
		&s.DeletedAt, &s.DeletedByName, &s.DeleteReason)
	s.Items = []model.SaleItem{}
	return s, err
}

func (s *Store) CreateSale(userID int64, terminal, invoiceCode string, items []model.SaleItemInput, paid int64) (model.Sale, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return model.Sale{}, err
	}
	defer tx.Rollback()

	type line struct {
		ticketID, price, qty, subtotal int64
	}
	var lines []line
	var total int64
	for _, it := range items {
		var price int64
		var name string
		var active bool
		err := tx.QueryRow("SELECT price, name, active FROM tickets WHERE id = ? AND deleted_at IS NULL", it.TicketID).Scan(&price, &name, &active)
		if err == sql.ErrNoRows {
			return model.Sale{}, model.Invalid("Tiket tidak ditemukan atau sudah dihapus")
		}
		if err == nil && !active {
			return model.Sale{}, model.Invalid("Tiket " + name + " sedang non-aktif dan tidak bisa dijual")
		}
		if err != nil {
			return model.Sale{}, err
		}
		sub := price * it.Qty
		lines = append(lines, line{it.TicketID, price, it.Qty, sub})
		total += sub
	}
	if paid < total {
		return model.Sale{}, model.Invalid("Uang yang dibayar kurang dari total")
	}

	now := s.Now()
	prefix, err := settingValue(tx, keyInvoicePrefix, model.DefaultInvoicePrefix)
	if err != nil {
		return model.Sale{}, err
	}
	invoice, err := nextInvoice(tx, prefix+invoiceCode, now)
	if err != nil {
		return model.Sale{}, err
	}
	res, err := tx.Exec(
		`INSERT INTO sales (invoice_no, user_id, terminal, total, paid, change_amount, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		invoice, userID, terminal, total, paid, paid-total, now.Format(timeLayout),
	)
	if err != nil {
		return model.Sale{}, err
	}
	saleID, _ := res.LastInsertId()
	for _, l := range lines {
		_, err := tx.Exec(
			"INSERT INTO sale_items (sale_id, ticket_id, price, qty, subtotal) VALUES (?, ?, ?, ?, ?)",
			saleID, l.ticketID, l.price, l.qty, l.subtotal,
		)
		if err != nil {
			return model.Sale{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return model.Sale{}, err
	}
	return s.GetSale(saleID)
}

func nextInvoice(tx *sql.Tx, base string, now time.Time) (string, error) {
	prefix := base + now.Format("20060102")
	var last string
	err := tx.QueryRow(
		`SELECT invoice_no FROM sales WHERE invoice_no LIKE ? ESCAPE '\' AND length(invoice_no) = ? ORDER BY invoice_no DESC LIMIT 1`,
		likePrefix(prefix), len(prefix)+4,
	).Scan(&last)
	seq := 1
	if err == nil {
		n, convErr := strconv.Atoi(strings.TrimPrefix(last, prefix))
		if convErr != nil {
			return "", convErr
		}
		seq = n + 1
	} else if err != sql.ErrNoRows {
		return "", err
	}
	if seq > 9999 {
		return "", model.Invalid("Nomor nota hari ini sudah mencapai 9999 untuk kode ini")
	}
	return fmt.Sprintf("%s%04d", prefix, seq), nil
}

func (s *Store) GetSale(id int64) (model.Sale, error) {
	sale, err := scanSale(s.DB.QueryRow("SELECT "+saleColumns+saleFrom+" WHERE s.id = ?", id))
	if err != nil {
		return sale, notFound(err)
	}
	sales := []model.Sale{sale}
	if err := s.loadItems(sales); err != nil {
		return sale, err
	}
	return sales[0], nil
}

// loadItems fills the Items of every sale in sales with a single query.
func (s *Store) loadItems(sales []model.Sale) error {
	if len(sales) == 0 {
		return nil
	}
	index := make(map[int64]int, len(sales))
	args := make([]any, len(sales))
	for i, sale := range sales {
		index[sale.ID] = i
		args[i] = sale.ID
	}
	rows, err := s.DB.Query(
		`SELECT i.sale_id, i.id, i.ticket_id, t.name, t.category, i.price, i.qty, i.subtotal
		 FROM sale_items i JOIN tickets t ON t.id = i.ticket_id
		 WHERE i.sale_id IN (?`+strings.Repeat(", ?", len(sales)-1)+`) ORDER BY i.id`, args...,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var saleID int64
		var it model.SaleItem
		if err := rows.Scan(&saleID, &it.ID, &it.TicketID, &it.TicketName, &it.Category, &it.Price, &it.Qty, &it.Subtotal); err != nil {
			return err
		}
		sale := &sales[index[saleID]]
		sale.Items = append(sale.Items, it)
	}
	return rows.Err()
}

func (s *Store) ListSales(from, to string, userID int64, deleted bool) ([]model.Sale, error) {
	start, end, err := dateRange(from, to)
	if err != nil {
		return nil, err
	}
	query := "SELECT " + saleColumns + saleFrom + " WHERE s.created_at >= ? AND s.created_at < ?"
	if deleted {
		query += " AND s.deleted_at IS NOT NULL"
	} else {
		query += " AND s.deleted_at IS NULL"
	}
	args := []any{start, end}
	if userID > 0 {
		query += " AND s.user_id = ?"
		args = append(args, userID)
	}
	rows, err := s.DB.Query(query+" ORDER BY s.created_at DESC, s.id DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sales := []model.Sale{}
	for rows.Next() {
		sale, err := scanSale(rows)
		if err != nil {
			return nil, err
		}
		sales = append(sales, sale)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sales, s.loadItems(sales)
}

func (s *Store) DeleteSale(id, userID int64, reason string) error {
	res, err := s.DB.Exec(
		"UPDATE sales SET deleted_at = ?, deleted_by = ?, delete_reason = ? WHERE id = ? AND deleted_at IS NULL",
		s.now(), userID, reason, id,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.GetSale(id); err != nil {
			return err
		}
		return model.Invalid("Nota sudah dihapus")
	}
	return nil
}

func dateRange(from, to string) (string, string, error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return "", "", model.Invalid("Tanggal awal tidak valid")
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return "", "", model.Invalid("Tanggal akhir tidak valid")
	}
	if end.Before(start) {
		return "", "", model.Invalid("Tanggal akhir harus setelah tanggal awal")
	}
	return start.Format(timeLayout), end.AddDate(0, 0, 1).Format(timeLayout), nil
}

func (s *Store) RecordPrint(saleID, userID int64, terminal string) error {
	_, err := s.DB.Exec(
		"INSERT INTO sale_prints (sale_id, user_id, terminal, printed_at) VALUES (?, ?, ?, ?)",
		saleID, userID, terminal, s.now(),
	)
	return err
}
