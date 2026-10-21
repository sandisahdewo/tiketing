package service

import (
	"encoding/base64"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"tiket/internal/auth"
	"tiket/internal/model"
	"tiket/internal/receipt"
	"tiket/internal/store"
)

const (
	defaultPageSize = 100
	maxPageSize     = 500
	maxNameLength   = 100
	maxQty          = 10000
	maxPrice        = 1_000_000_000_00
)

var (
	usernamePattern    = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)
	invoicePartPattern = regexp.MustCompile(`^[A-Z0-9]{0,6}$`)
)

func ValidateInvoiceCode(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if !invoicePartPattern.MatchString(code) || len(code) > 4 {
		return "", model.Invalid("Kode nota maksimal 4 karakter huruf/angka")
	}
	return code, nil
}

type Local struct {
	Store    *store.Store
	Sessions *auth.Sessions
	Version  string
}

func NewLocal(st *store.Store, version string) *Local {
	return &Local{Store: st, Sessions: auth.NewSessions(), Version: version}
}

func (l *Local) user(token string, roles ...string) (model.User, error) {
	id, err := l.Sessions.Lookup(token)
	if err != nil {
		return model.User{}, err
	}
	u, err := l.Store.GetUser(id)
	if err == model.ErrNotFound {
		l.Sessions.Delete(token)
		return model.User{}, model.ErrUnauthorized
	}
	if err != nil {
		return model.User{}, err
	}
	if len(roles) == 0 {
		return u, nil
	}
	for _, r := range roles {
		if u.Role == r {
			return u, nil
		}
	}
	return model.User{}, model.ErrForbidden
}

func (l *Local) Status() (model.Status, error) {
	n, err := l.Store.CountUsers()
	return model.Status{Version: l.Version, HasUsers: n > 0}, err
}

func (l *Local) CreateFirstAdmin(in model.UserInput) error {
	n, err := l.Store.CountUsers()
	if err != nil {
		return err
	}
	if n > 0 {
		return model.Invalid("Admin pertama sudah dibuat")
	}
	in.Role = model.RoleAdmin
	if err := validateUser(&in, true); err != nil {
		return err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return err
	}
	_, err = l.Store.CreateUser(in.Username, in.FullName, in.Role, hash)
	return err
}

func (l *Local) Login(username, password string) (model.Session, error) {
	u, hash, err := l.Store.GetUserForLogin(strings.ToLower(strings.TrimSpace(username)))
	if err == model.ErrNotFound || (err == nil && !auth.CheckPassword(hash, password)) {
		return model.Session{}, model.Invalid("Username atau password salah")
	}
	if err != nil {
		return model.Session{}, err
	}
	token, err := l.Sessions.Create(u.ID)
	if err != nil {
		return model.Session{}, err
	}
	return model.Session{Token: token, User: u}, nil
}

func (l *Local) Logout(token string) error {
	l.Sessions.Delete(token)
	return nil
}

func (l *Local) Me(token string) (model.User, error) {
	return l.user(token)
}

func (l *Local) ListTickets(token, search string, deleted, activeOnly bool) ([]model.Ticket, error) {
	u, err := l.user(token)
	if err != nil {
		return nil, err
	}
	// Cashiers only ever see what they can sell.
	if u.Role != model.RoleAdmin {
		if deleted {
			return nil, model.ErrForbidden
		}
		activeOnly = true
	}
	return l.Store.ListTickets(search, deleted, activeOnly)
}

func (l *Local) SearchTickets(token string, q model.TicketQuery) (model.TicketPage, error) {
	u, err := l.user(token)
	if err != nil {
		return model.TicketPage{}, err
	}
	switch q.Status {
	case "", model.TicketAll:
		q.Status = model.TicketAll
	case model.TicketActive, model.TicketInactive, model.TicketDeleted:
	default:
		return model.TicketPage{}, model.Invalid("Status tiket tidak valid")
	}
	if q.Category != "" && q.Category != model.CategoryTicket && q.Category != model.CategoryTopup {
		return model.TicketPage{}, model.Invalid("Kategori tidak valid")
	}
	// Cashiers only ever see what they can sell.
	if u.Role != model.RoleAdmin {
		if q.Status == model.TicketDeleted {
			return model.TicketPage{}, model.ErrForbidden
		}
		q.Status = model.TicketActive
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = defaultPageSize
	}
	q.PageSize = min(q.PageSize, maxPageSize)
	return l.Store.SearchTickets(q)
}

func (l *Local) SetTicketActive(token string, id int64, active bool) error {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return err
	}
	return l.Store.SetTicketActive(id, active)
}

func (l *Local) SaveTicket(token string, in model.TicketInput) (model.Ticket, error) {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return model.Ticket{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return model.Ticket{}, model.Invalid("Nama tiket wajib diisi")
	}
	if utf8.RuneCountInString(in.Name) > maxNameLength {
		return model.Ticket{}, model.Invalid("Nama tiket terlalu panjang (maksimal 100 karakter)")
	}
	if in.Price < 0 || in.Price > maxPrice {
		return model.Ticket{}, model.Invalid("Harga tidak valid")
	}
	if in.Category == "" {
		in.Category = model.CategoryTicket
	}
	if in.Category != model.CategoryTicket && in.Category != model.CategoryTopup {
		return model.Ticket{}, model.Invalid("Kategori tidak valid")
	}
	if in.Color == "" {
		in.Color = model.DefaultColor
	}
	if !slices.Contains(model.TicketColors, in.Color) {
		return model.Ticket{}, model.Invalid("Warna tidak valid")
	}
	if in.ID == 0 {
		return l.Store.CreateTicket(in)
	}
	return l.Store.UpdateTicket(in)
}

func (l *Local) DeleteTicket(token string, id int64) error {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return err
	}
	return l.Store.DeleteTicket(id)
}

func (l *Local) RestoreTicket(token string, id int64) error {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return err
	}
	return l.Store.RestoreTicket(id)
}

func (l *Local) ListUsers(token string) ([]model.User, error) {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return nil, err
	}
	return l.Store.ListUsers()
}

func validateUser(in *model.UserInput, isNew bool) error {
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	in.FullName = strings.TrimSpace(in.FullName)
	if !usernamePattern.MatchString(in.Username) {
		return model.Invalid("Username 3-32 karakter: huruf kecil, angka, titik, minus, atau garis bawah")
	}
	if in.FullName == "" {
		return model.Invalid("Nama lengkap wajib diisi")
	}
	if utf8.RuneCountInString(in.FullName) > maxNameLength {
		return model.Invalid("Nama lengkap terlalu panjang")
	}
	if in.Role != model.RoleAdmin && in.Role != model.RoleKasir {
		return model.Invalid("Role harus admin atau kasir")
	}
	if (isNew || in.Password != "") && utf8.RuneCountInString(in.Password) < 4 {
		return model.Invalid("Password minimal 4 karakter")
	}
	return nil
}

func (l *Local) SaveUser(token string, in model.UserInput) (model.User, error) {
	me, err := l.user(token, model.RoleAdmin)
	if err != nil {
		return model.User{}, err
	}
	if err := validateUser(&in, in.ID == 0); err != nil {
		return model.User{}, err
	}
	var hash string
	if in.Password != "" {
		if hash, err = auth.HashPassword(in.Password); err != nil {
			return model.User{}, err
		}
	}
	if in.ID == 0 {
		return l.Store.CreateUser(in.Username, in.FullName, in.Role, hash)
	}

	existing, err := l.Store.GetUser(in.ID)
	if err != nil {
		return model.User{}, err
	}
	if existing.Role == model.RoleAdmin && in.Role != model.RoleAdmin {
		if in.ID == me.ID {
			return model.User{}, model.Invalid("Anda tidak bisa menurunkan role akun sendiri")
		}
		if err := l.ensureAnotherAdmin(); err != nil {
			return model.User{}, err
		}
	}
	u, err := l.Store.UpdateUser(in.ID, in.Username, in.FullName, in.Role)
	if err != nil {
		return model.User{}, err
	}
	if hash != "" {
		if err := l.Store.SetPassword(in.ID, hash); err != nil {
			return model.User{}, err
		}
		if in.ID != me.ID {
			l.Sessions.DeleteUser(in.ID)
		}
	}
	return u, nil
}

func (l *Local) ensureAnotherAdmin() error {
	n, err := l.Store.CountAdmins()
	if err != nil {
		return err
	}
	if n <= 1 {
		return model.Invalid("Harus ada minimal satu admin")
	}
	return nil
}

func (l *Local) DeleteUser(token string, id int64) error {
	me, err := l.user(token, model.RoleAdmin)
	if err != nil {
		return err
	}
	if id == me.ID {
		return model.Invalid("Anda tidak bisa menghapus akun sendiri")
	}
	target, err := l.Store.GetUser(id)
	if err != nil {
		return err
	}
	if target.Role == model.RoleAdmin {
		if err := l.ensureAnotherAdmin(); err != nil {
			return err
		}
	}
	if err := l.Store.DeleteUser(id); err != nil {
		return err
	}
	l.Sessions.DeleteUser(id)
	return nil
}

func (l *Local) CreateSale(token string, in model.SaleInput) (model.Sale, error) {
	u, err := l.user(token)
	if err != nil {
		return model.Sale{}, err
	}
	merged := map[int64]int64{}
	var order []int64
	for _, it := range in.Items {
		if it.Qty <= 0 || it.Qty > maxQty {
			return model.Sale{}, model.Invalid("Jumlah tiket tidak valid")
		}
		if _, ok := merged[it.TicketID]; !ok {
			order = append(order, it.TicketID)
		}
		merged[it.TicketID] += it.Qty
	}
	if len(order) == 0 {
		return model.Sale{}, model.Invalid("Keranjang masih kosong")
	}
	items := make([]model.SaleItemInput, 0, len(order))
	for _, id := range order {
		if merged[id] > maxQty {
			return model.Sale{}, model.Invalid("Jumlah tiket tidak valid")
		}
		items = append(items, model.SaleItemInput{TicketID: id, Qty: merged[id]})
	}
	if in.Paid < 0 || in.Paid > maxPrice*maxQty {
		return model.Sale{}, model.Invalid("Jumlah bayar tidak valid")
	}
	code, err := ValidateInvoiceCode(in.InvoiceCode)
	if err != nil {
		return model.Sale{}, err
	}
	return l.Store.CreateSale(u.ID, cleanTerminal(in.Terminal), code, items, in.Paid)
}

func cleanTerminal(terminal string) string {
	terminal = strings.TrimSpace(terminal)
	if terminal == "" {
		return "-"
	}
	return terminal
}

func (l *Local) GetSale(token string, id int64) (model.Sale, error) {
	_, sale, err := l.ownSale(token, id)
	return sale, err
}

func (l *Local) ownSale(token string, id int64) (model.User, model.Sale, error) {
	u, err := l.user(token)
	if err != nil {
		return u, model.Sale{}, err
	}
	sale, err := l.Store.GetSale(id)
	if err != nil {
		return u, sale, err
	}
	if u.Role != model.RoleAdmin && sale.UserID != u.ID {
		return u, model.Sale{}, model.ErrForbidden
	}
	return u, sale, nil
}

func (l *Local) RecordPrint(token string, in model.PrintInput) error {
	u, sale, err := l.ownSale(token, in.SaleID)
	if err != nil {
		return err
	}
	if sale.DeletedAt != nil {
		return model.Invalid("Nota sudah dihapus, tidak bisa dicetak")
	}
	return l.Store.RecordPrint(in.SaleID, u.ID, cleanTerminal(in.Terminal))
}

// today is the server's current date; cashiers only see reports for it.
func (l *Local) today() string {
	return l.Store.Now().Format("2006-01-02")
}

// salesScope applies the role rules to a sale query: cashiers only see their
// own sales of today and never the deleted ones. It returns the user filter.
func (l *Local) salesScope(token string, q *model.SaleQuery) (int64, error) {
	u, err := l.user(token)
	if err != nil {
		return 0, err
	}
	if u.Role == model.RoleAdmin {
		return 0, nil
	}
	if q.Deleted {
		return 0, model.ErrForbidden
	}
	q.From, q.To = l.today(), l.today()
	return u.ID, nil
}

func (l *Local) ListSales(token string, q model.SaleQuery) ([]model.Sale, error) {
	userID, err := l.salesScope(token, &q)
	if err != nil {
		return nil, err
	}
	return l.Store.ListSales(q, userID)
}

func (l *Local) SearchSales(token string, q model.SalePageQuery) (model.SalePage, error) {
	userID, err := l.salesScope(token, &q.SaleQuery)
	if err != nil {
		return model.SalePage{}, err
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = defaultPageSize
	}
	q.PageSize = min(q.PageSize, maxPageSize)
	return l.Store.SearchSales(q, userID)
}

func (l *Local) DeleteSale(token string, in model.DeleteSaleInput) error {
	u, err := l.user(token, model.RoleAdmin)
	if err != nil {
		return err
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return model.Invalid("Alasan hapus nota wajib diisi")
	}
	if utf8.RuneCountInString(reason) > 200 {
		return model.Invalid("Alasan maksimal 200 karakter")
	}
	return l.Store.DeleteSale(in.ID, u.ID, reason)
}

func (l *Local) Report(token string, q model.ReportQuery) (model.Report, error) {
	u, err := l.user(token)
	if err != nil {
		return model.Report{}, err
	}
	var userID int64
	if u.Role != model.RoleAdmin {
		userID = u.ID
		q = model.ReportQuery{Period: "daily", From: l.today(), To: l.today()}
	}
	return l.Store.Report(q, userID)
}

func (l *Local) GetSettings(token string) (model.Settings, error) {
	if _, err := l.user(token); err != nil {
		return model.Settings{}, err
	}
	return l.Store.GetSettings()
}

func (l *Local) SaveSettings(token string, in model.Settings) error {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return err
	}
	in.BusinessName = strings.TrimSpace(in.BusinessName)
	in.Address = strings.TrimSpace(in.Address)
	in.Footer = strings.TrimSpace(in.Footer)
	in.InvoicePrefix = strings.ToUpper(strings.TrimSpace(in.InvoicePrefix))
	in.PurposeWeekday = strings.TrimSpace(in.PurposeWeekday)
	in.PurposeWeekend = strings.TrimSpace(in.PurposeWeekend)
	if in.InvoicePrefix == "" || !invoicePartPattern.MatchString(in.InvoicePrefix) {
		return model.Invalid("Prefix nota 1-6 karakter huruf/angka, contoh PNY")
	}
	if in.PurposeWeekday == "" || in.PurposeWeekend == "" {
		return model.Invalid("Purpose weekday dan weekend wajib diisi")
	}
	if utf8.RuneCountInString(in.PurposeWeekday) > 40 || utf8.RuneCountInString(in.PurposeWeekend) > 40 {
		return model.Invalid("Purpose maksimal 40 karakter")
	}
	return l.Store.SaveSettings(in)
}

func (l *Local) GetLogo(token string) (string, error) {
	if _, err := l.user(token); err != nil {
		return "", err
	}
	return l.Store.GetLogo()
}

func (l *Local) SaveLogo(token, base64Data string) error {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return err
	}
	if base64Data != "" {
		data, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return model.Invalid("Data logo tidak valid")
		}
		if err := receipt.ValidateLogo(data); err != nil {
			return model.Invalid(err.Error())
		}
	}
	return l.Store.SaveLogo(base64Data)
}
