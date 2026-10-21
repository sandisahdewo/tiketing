package service

import (
	"sync"

	"tiket/internal/applog"
	"tiket/internal/model"
	"tiket/internal/money"
)

// Logged wraps a Service and writes every action and its outcome to the
// application log. Plain reads are only logged when they fail. Failed actions
// also log their input (with passwords masked) so they can be reproduced.
// Every line carries op (one user action) and sess (one login).
type Logged struct {
	svc   Service
	op    string // fixed by WithOp; otherwise every call is its own op
	users *users
}

type users struct {
	mu     sync.Mutex
	byName map[string]string // token -> username
}

func NewLogged(svc Service) *Logged {
	return &Logged{svc: svc, users: &users{byName: map[string]string{}}}
}

// opScoped is implemented by services that forward the op id (Remote).
type opScoped interface {
	WithOp(op string) Service
}

// WithOp returns a Logged whose calls all belong to the user action op.
func (l *Logged) WithOp(op string) *Logged {
	c := *l
	c.op = op
	if r, ok := l.svc.(opScoped); ok {
		c.svc = r.WithOp(op)
	}
	return &c
}

// call is one logged step of an action.
type call struct {
	l     *Logged
	op    string
	token string
	svc   Service
}

func (l *Logged) begin(token string) call {
	if l.op != "" {
		return call{l: l, op: l.op, token: token, svc: l.svc}
	}
	scoped := l.WithOp(applog.NewOpID())
	return call{l: l, op: scoped.op, token: token, svc: scoped.svc}
}

func (u *users) get(token string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if name, ok := u.byName[token]; ok {
		return name
	}
	return "-"
}

func (u *users) set(token, name string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if name == "" {
		delete(u.byName, token)
	} else {
		u.byName[token] = name
	}
}

// log records the action; input is only written when it failed.
func (c call) log(action string, err error, input any, attrs ...any) {
	attrs = append([]any{"op", c.op, "sess", applog.SessionID(c.token), "user", c.l.users.get(c.token)}, attrs...)
	if err != nil && input != nil {
		attrs = append(attrs, "input", applog.Payload(input))
	}
	applog.Action(action, err, attrs...)
}

// logError records a read only when it failed.
func (c call) logError(action string, err error, input any, attrs ...any) {
	if err != nil {
		c.log(action, err, input, attrs...)
	}
}

func (l *Logged) Status() (model.Status, error) {
	c := l.begin("")
	out, err := c.svc.Status()
	c.logError("Status", err, nil)
	return out, err
}

func (l *Logged) CreateFirstAdmin(in model.UserInput) error {
	c := l.begin("")
	err := c.svc.CreateFirstAdmin(in)
	c.log("CreateFirstAdmin", err, in, "username", in.Username)
	return err
}

func (l *Logged) Login(username, password string) (model.Session, error) {
	c := l.begin("")
	out, err := c.svc.Login(username, password)
	if err == nil {
		l.users.set(out.Token, out.User.Username)
		c.token = out.Token
	}
	c.log("Login", err, nil, "username", username)
	return out, err
}

func (l *Logged) Logout(token string) error {
	c := l.begin(token)
	err := c.svc.Logout(token)
	c.log("Logout", err, nil)
	l.users.set(token, "")
	return err
}

func (l *Logged) Me(token string) (model.User, error) {
	c := l.begin(token)
	out, err := c.svc.Me(token)
	c.logError("Me", err, nil)
	return out, err
}

func (l *Logged) ListTickets(token, search string, deleted, activeOnly bool) ([]model.Ticket, error) {
	c := l.begin(token)
	out, err := c.svc.ListTickets(token, search, deleted, activeOnly)
	c.logError("ListTickets", err, map[string]any{"search": search, "deleted": deleted, "activeOnly": activeOnly})
	return out, err
}

func (l *Logged) SaveTicket(token string, in model.TicketInput) (model.Ticket, error) {
	c := l.begin(token)
	out, err := c.svc.SaveTicket(token, in)
	c.log("SaveTicket", err, in, "id", out.ID, "name", in.Name, "price", money.Format(in.Price), "category", in.Category)
	return out, err
}

func (l *Logged) DeleteTicket(token string, id int64) error {
	c := l.begin(token)
	err := c.svc.DeleteTicket(token, id)
	c.log("DeleteTicket", err, nil, "id", id)
	return err
}

func (l *Logged) RestoreTicket(token string, id int64) error {
	c := l.begin(token)
	err := c.svc.RestoreTicket(token, id)
	c.log("RestoreTicket", err, nil, "id", id)
	return err
}

// SearchTickets runs on every scroll, page change and search, so it is only
// logged when it fails.
func (l *Logged) SearchTickets(token string, q model.TicketQuery) (model.TicketPage, error) {
	c := l.begin(token)
	out, err := c.svc.SearchTickets(token, q)
	c.logError("SearchTickets", err, q)
	return out, err
}

func (l *Logged) SetTicketActive(token string, id int64, active bool) error {
	c := l.begin(token)
	err := c.svc.SetTicketActive(token, id, active)
	c.log("SetTicketActive", err, nil, "id", id, "active", active)
	return err
}

func (l *Logged) ListUsers(token string) ([]model.User, error) {
	c := l.begin(token)
	out, err := c.svc.ListUsers(token)
	c.logError("ListUsers", err, nil)
	return out, err
}

func (l *Logged) SaveUser(token string, in model.UserInput) (model.User, error) {
	c := l.begin(token)
	out, err := c.svc.SaveUser(token, in)
	c.log("SaveUser", err, in, "id", out.ID, "username", in.Username, "role", in.Role, "passwordChanged", in.Password != "")
	return out, err
}

func (l *Logged) DeleteUser(token string, id int64) error {
	c := l.begin(token)
	err := c.svc.DeleteUser(token, id)
	c.log("DeleteUser", err, nil, "id", id)
	return err
}

func (l *Logged) CreateSale(token string, in model.SaleInput) (model.Sale, error) {
	c := l.begin(token)
	out, err := c.svc.CreateSale(token, in)
	c.log("CreateSale", err, in, "invoice", out.InvoiceNo, "items", len(in.Items), "total", money.Format(out.Total), "paid", money.Format(in.Paid))
	return out, err
}

func (l *Logged) GetSale(token string, id int64) (model.Sale, error) {
	c := l.begin(token)
	out, err := c.svc.GetSale(token, id)
	c.logError("GetSale", err, nil, "id", id)
	return out, err
}

func (l *Logged) ListSales(token string, q model.SaleQuery) ([]model.Sale, error) {
	c := l.begin(token)
	out, err := c.svc.ListSales(token, q)
	c.log("ListSales", err, q, "from", q.From, "to", q.To, "deleted", q.Deleted, "count", len(out))
	return out, err
}

// SearchSales runs on every page change and (debounced) keystroke, so it is
// only logged when it fails.
func (l *Logged) SearchSales(token string, q model.SalePageQuery) (model.SalePage, error) {
	c := l.begin(token)
	out, err := c.svc.SearchSales(token, q)
	c.logError("SearchSales", err, q)
	return out, err
}

func (l *Logged) DeleteSale(token string, in model.DeleteSaleInput) error {
	c := l.begin(token)
	err := c.svc.DeleteSale(token, in)
	c.log("DeleteSale", err, in, "id", in.ID, "reason", in.Reason)
	return err
}

func (l *Logged) Report(token string, q model.ReportQuery) (model.Report, error) {
	c := l.begin(token)
	out, err := c.svc.Report(token, q)
	c.log("Report", err, q, "period", out.Period, "from", out.From, "to", out.To, "total", money.Format(out.Total))
	return out, err
}

func (l *Logged) RecordPrint(token string, in model.PrintInput) error {
	c := l.begin(token)
	err := c.svc.RecordPrint(token, in)
	c.log("RecordPrint", err, in, "sale", in.SaleID)
	return err
}

func (l *Logged) GetSettings(token string) (model.Settings, error) {
	c := l.begin(token)
	out, err := c.svc.GetSettings(token)
	c.logError("GetSettings", err, nil)
	return out, err
}

func (l *Logged) SaveSettings(token string, in model.Settings) error {
	c := l.begin(token)
	err := c.svc.SaveSettings(token, in)
	c.log("SaveSettings", err, in, "business", in.BusinessName, "invoicePrefix", in.InvoicePrefix)
	return err
}

func (l *Logged) GetLogo(token string) (string, error) {
	c := l.begin(token)
	out, err := c.svc.GetLogo(token)
	c.logError("GetLogo", err, nil)
	return out, err
}

func (l *Logged) SaveLogo(token, base64Data string) error {
	c := l.begin(token)
	err := c.svc.SaveLogo(token, base64Data)
	c.log("SaveLogo", err, nil, "removed", base64Data == "", "bytes", len(base64Data))
	return err
}
