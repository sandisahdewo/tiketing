package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tiket/internal/model"
)

type Remote struct {
	BaseURL string
	Version string
	Client  *http.Client
	OpID    string
}

// WithOp returns a copy whose calls send op in X-Op-ID, so the server logs
// them under the same action id as this PC.
func (r *Remote) WithOp(op string) Service {
	c := *r
	c.OpID = op
	return &c
}

func NormalizeServerURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	return strings.TrimRight(raw, "/")
}

func NewRemote(baseURL, version string) *Remote {
	return &Remote{
		BaseURL: NormalizeServerURL(baseURL),
		Version: version,
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

type remoteError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func (r *Remote) do(req *http.Request, out any) error {
	resp, err := r.Client.Do(req)
	if err != nil {
		return fmt.Errorf("Tidak bisa terhubung ke server %s. Pastikan PC server menyala dan satu jaringan.", r.BaseURL)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e remoteError
		if err := json.NewDecoder(resp.Body).Decode(&e); err != nil || e.Error == "" {
			return fmt.Errorf("Server membalas dengan status %d", resp.StatusCode)
		}
		switch e.Code {
		case "unauthorized":
			return model.ErrUnauthorized
		case "forbidden":
			return model.ErrForbidden
		case "not_found":
			return model.ErrNotFound
		case "version_mismatch":
			return &model.VersionMismatchError{Message: e.Error}
		case "invalid":
			return model.Invalid(e.Error)
		}
		return fmt.Errorf("%s", e.Error)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("Balasan server tidak valid: %w", err)
	}
	return nil
}

func (r *Remote) call(name, token string, in, out any) error {
	if in == nil {
		in = struct{}{}
	}
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, r.BaseURL+"/api/"+name, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Version", r.Version)
	if r.OpID != "" {
		req.Header.Set("X-Op-ID", r.OpID)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return r.do(req, out)
}

func (r *Remote) Status() (model.Status, error) {
	var st model.Status
	req, err := http.NewRequest(http.MethodGet, r.BaseURL+"/api/status", nil)
	if err != nil {
		return st, err
	}
	return st, r.do(req, &st)
}

type idRequest struct {
	ID int64 `json:"id"`
}

func (r *Remote) CreateFirstAdmin(in model.UserInput) error {
	return r.call("createFirstAdmin", "", in, nil)
}

func (r *Remote) Login(username, password string) (model.Session, error) {
	var s model.Session
	err := r.call("login", "", map[string]string{"username": username, "password": password}, &s)
	return s, err
}

func (r *Remote) Logout(token string) error {
	return r.call("logout", token, nil, nil)
}

func (r *Remote) Me(token string) (model.User, error) {
	var u model.User
	return u, r.call("me", token, nil, &u)
}

func (r *Remote) ListTickets(token, search string, deleted, activeOnly bool) ([]model.Ticket, error) {
	var out []model.Ticket
	in := map[string]any{"search": search, "deleted": deleted, "activeOnly": activeOnly}
	return out, r.call("listTickets", token, in, &out)
}

func (r *Remote) SaveTicket(token string, in model.TicketInput) (model.Ticket, error) {
	var out model.Ticket
	return out, r.call("saveTicket", token, in, &out)
}

func (r *Remote) DeleteTicket(token string, id int64) error {
	return r.call("deleteTicket", token, idRequest{id}, nil)
}

func (r *Remote) RestoreTicket(token string, id int64) error {
	return r.call("restoreTicket", token, idRequest{id}, nil)
}

func (r *Remote) SearchTickets(token string, q model.TicketQuery) (model.TicketPage, error) {
	var out model.TicketPage
	return out, r.call("searchTickets", token, q, &out)
}

func (r *Remote) SetTicketActive(token string, id int64, active bool) error {
	return r.call("setTicketActive", token, map[string]any{"id": id, "active": active}, nil)
}

func (r *Remote) ListUsers(token string) ([]model.User, error) {
	var out []model.User
	return out, r.call("listUsers", token, nil, &out)
}

func (r *Remote) SaveUser(token string, in model.UserInput) (model.User, error) {
	var out model.User
	return out, r.call("saveUser", token, in, &out)
}

func (r *Remote) DeleteUser(token string, id int64) error {
	return r.call("deleteUser", token, idRequest{id}, nil)
}

func (r *Remote) CreateSale(token string, in model.SaleInput) (model.Sale, error) {
	var out model.Sale
	return out, r.call("createSale", token, in, &out)
}

func (r *Remote) GetSale(token string, id int64) (model.Sale, error) {
	var out model.Sale
	return out, r.call("getSale", token, idRequest{id}, &out)
}

func (r *Remote) ListSales(token string, q model.SaleQuery) ([]model.Sale, error) {
	var out []model.Sale
	return out, r.call("listSales", token, q, &out)
}

func (r *Remote) SearchSales(token string, q model.SalePageQuery) (model.SalePage, error) {
	var out model.SalePage
	return out, r.call("searchSales", token, q, &out)
}

func (r *Remote) DeleteSale(token string, in model.DeleteSaleInput) error {
	return r.call("deleteSale", token, in, nil)
}

func (r *Remote) Report(token string, q model.ReportQuery) (model.Report, error) {
	var out model.Report
	return out, r.call("report", token, q, &out)
}

func (r *Remote) ReportFilters(token string) (model.ReportFilters, error) {
	var out model.ReportFilters
	return out, r.call("reportFilters", token, nil, &out)
}

func (r *Remote) ListCounters(token string) ([]model.Counter, error) {
	var out []model.Counter
	return out, r.call("listCounters", token, nil, &out)
}

func (r *Remote) SaveCounter(token string, in model.CounterInput) (model.Counter, error) {
	var out model.Counter
	return out, r.call("saveCounter", token, in, &out)
}

func (r *Remote) SetCounterActive(token string, id int64, active bool) error {
	return r.call("setCounterActive", token, map[string]any{"id": id, "active": active}, nil)
}

func (r *Remote) AssignCounter(token string, in model.AssignCounterInput) error {
	return r.call("assignCounter", token, in, nil)
}

func (r *Remote) DeviceCounter(deviceID string) (model.Counter, error) {
	var out model.Counter
	return out, r.call("deviceCounter", "", map[string]string{"deviceId": deviceID}, &out)
}

func (r *Remote) GetSettings(token string) (model.Settings, error) {
	var out model.Settings
	return out, r.call("getSettings", token, nil, &out)
}

func (r *Remote) SaveSettings(token string, in model.Settings) error {
	return r.call("saveSettings", token, in, nil)
}

func (r *Remote) RecordPrint(token string, in model.PrintInput) error {
	return r.call("recordPrint", token, in, nil)
}

type logoBody struct {
	Data string `json:"data"`
}

func (r *Remote) GetLogo(token string) (string, error) {
	var out logoBody
	return out.Data, r.call("getLogo", token, nil, &out)
}

func (r *Remote) SaveLogo(token, base64Data string) error {
	return r.call("saveLogo", token, logoBody{Data: base64Data}, nil)
}
