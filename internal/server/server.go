package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tiket/internal/applog"
	"tiket/internal/model"
	"tiket/internal/money"
	"tiket/internal/service"
	"tiket/internal/version"
)

const (
	HeaderVersion = "X-App-Version"
	maxBodyBytes  = 4 << 20
)

type ErrorBody struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type IDRequest struct {
	ID int64 `json:"id"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ListTicketsRequest struct {
	Search     string `json:"search"`
	Deleted    bool   `json:"deleted"`
	ActiveOnly bool   `json:"activeOnly"`
}

type SetActiveRequest struct {
	ID     int64 `json:"id"`
	Active bool  `json:"active"`
}

type Empty struct{}

// Requests from client PCs are logged on the server under the client's
// X-Op-ID and the session derived from the token. Reads are only logged when they fail. Successful actions log
// a short summary; failed ones log the whole body with passwords masked.
const HeaderOpID = "X-Op-ID"

var (
	readRoutes  = map[string]bool{"reportFilters": true, "searchSales": true, "searchTickets": true, "me": true, "listTickets": true, "getSale": true, "listUsers": true, "getSettings": true, "getLogo": true}
	summaryKeys = []string{"id", "active", "saleId", "username", "role", "name", "reason", "period", "from", "to", "deleted", "category", "userId", "terminal"}
)

func requestUser(svc service.Service, token string, out any) string {
	if sess, ok := out.(model.Session); ok && sess.Token != "" {
		return sess.User.Username
	}
	if token == "" {
		return "-"
	}
	u, err := svc.Me(token)
	if err != nil {
		return "-"
	}
	return u.Username
}

// summary picks the identifying fields of a request and its result.
func summary(body []byte, out any) []any {
	var attrs []any
	var in map[string]any
	if json.Unmarshal(body, &in) == nil {
		for _, k := range summaryKeys {
			if v, ok := in[k]; ok && v != "" {
				attrs = append(attrs, k, v)
			}
		}
	}
	switch o := out.(type) {
	case model.Sale:
		attrs = append(attrs, "invoice", o.InvoiceNo, "total", money.Format(o.Total))
	case []model.Sale:
		attrs = append(attrs, "count", len(o))
	case model.SalePage:
		attrs = append(attrs, "count", o.Count)
	case model.Report:
		attrs = append(attrs, "total", money.Format(o.Total))
	case model.Ticket:
		attrs = append(attrs, "ticket", o.ID)
	case model.User:
		attrs = append(attrs, "userId", o.ID)
	}
	return attrs
}

type LogoBody struct {
	Data string `json:"data"`
}

func Handler(svc service.Service, serverVersion string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		st, err := svc.Status()
		respond(w, st, err)
	})

	route := func(name string, fn func(token string, body []byte) (any, error)) {
		mux.HandleFunc("POST /api/"+name, func(w http.ResponseWriter, r *http.Request) {
			action := "remote:" + name
			op := r.Header.Get(HeaderOpID)
			if op == "" {
				op = applog.NewOpID()
			}
			defer func() {
				if p := recover(); p != nil {
					applog.Action(action, fmt.Errorf("panic: %v", p), "op", op, "client", r.RemoteAddr)
					writeError(w, http.StatusInternalServerError, ErrorBody{Code: "internal", Error: "Terjadi kesalahan di server"})
				}
			}()
			clientVersion := r.Header.Get(HeaderVersion)
			if !version.Compatible(clientVersion, serverVersion) {
				applog.Action(action, model.ErrVersionMismatch, "op", op, "client", r.RemoteAddr, "clientVersion", clientVersion)
				writeError(w, http.StatusConflict, ErrorBody{
					Code:  "version_mismatch",
					Error: "Versi aplikasi (" + clientVersion + ") tidak cocok dengan server (" + serverVersion + "). Silakan update aplikasi.",
				})
				return
			}
			body := make([]byte, 0)
			if r.ContentLength != 0 {
				dec := http.MaxBytesReader(w, r.Body, maxBodyBytes)
				var raw json.RawMessage
				if err := json.NewDecoder(dec).Decode(&raw); err != nil {
					applog.Action(action, model.Invalid("Request tidak valid"), "op", op, "client", r.RemoteAddr, "decodeError", err.Error())
					writeError(w, http.StatusBadRequest, ErrorBody{Code: "invalid", Error: "Request tidak valid"})
					return
				}
				body = raw
			}
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			// Look the user up before the call so logout still knows who it was.
			user := ""
			if name != "login" {
				user = requestUser(svc, token, nil)
			}
			start := time.Now()
			out, err := fn(token, body)
			if !readRoutes[name] || err != nil {
				if user == "" {
					user = requestUser(svc, token, out)
				}
				sess := token
				if s, ok := out.(model.Session); ok && s.Token != "" {
					sess = s.Token
				}
				attrs := []any{"op", op, "sess", applog.SessionID(sess), "user", user, "client", r.RemoteAddr, "ms", time.Since(start).Milliseconds()}
				if err != nil && len(body) > 0 {
					attrs = append(attrs, "input", applog.PayloadJSON(body))
				} else {
					attrs = append(attrs, summary(body, out)...)
				}
				applog.Action(action, err, attrs...)
			}
			respond(w, out, err)
		})
	}

	route("createFirstAdmin", call(func(_ string, in model.UserInput) (any, error) {
		return Empty{}, svc.CreateFirstAdmin(in)
	}))
	route("login", call(func(_ string, in LoginRequest) (any, error) {
		return svc.Login(in.Username, in.Password)
	}))
	route("logout", call(func(t string, _ Empty) (any, error) {
		return Empty{}, svc.Logout(t)
	}))
	route("me", call(func(t string, _ Empty) (any, error) {
		return svc.Me(t)
	}))
	route("listTickets", call(func(t string, in ListTicketsRequest) (any, error) {
		return svc.ListTickets(t, in.Search, in.Deleted, in.ActiveOnly)
	}))
	route("searchTickets", call(func(t string, in model.TicketQuery) (any, error) {
		return svc.SearchTickets(t, in)
	}))
	route("setTicketActive", call(func(t string, in SetActiveRequest) (any, error) {
		return Empty{}, svc.SetTicketActive(t, in.ID, in.Active)
	}))
	route("saveTicket", call(func(t string, in model.TicketInput) (any, error) {
		return svc.SaveTicket(t, in)
	}))
	route("deleteTicket", call(func(t string, in IDRequest) (any, error) {
		return Empty{}, svc.DeleteTicket(t, in.ID)
	}))
	route("restoreTicket", call(func(t string, in IDRequest) (any, error) {
		return Empty{}, svc.RestoreTicket(t, in.ID)
	}))
	route("listUsers", call(func(t string, _ Empty) (any, error) {
		return svc.ListUsers(t)
	}))
	route("saveUser", call(func(t string, in model.UserInput) (any, error) {
		return svc.SaveUser(t, in)
	}))
	route("deleteUser", call(func(t string, in IDRequest) (any, error) {
		return Empty{}, svc.DeleteUser(t, in.ID)
	}))
	route("createSale", call(func(t string, in model.SaleInput) (any, error) {
		return svc.CreateSale(t, in)
	}))
	route("getSale", call(func(t string, in IDRequest) (any, error) {
		return svc.GetSale(t, in.ID)
	}))
	route("listSales", call(func(t string, in model.SaleQuery) (any, error) {
		return svc.ListSales(t, in)
	}))
	route("searchSales", call(func(t string, in model.SalePageQuery) (any, error) {
		return svc.SearchSales(t, in)
	}))
	route("deleteSale", call(func(t string, in model.DeleteSaleInput) (any, error) {
		return Empty{}, svc.DeleteSale(t, in)
	}))
	route("report", call(func(t string, in model.ReportQuery) (any, error) {
		return svc.Report(t, in)
	}))
	route("reportFilters", call(func(t string, _ Empty) (any, error) {
		return svc.ReportFilters(t)
	}))
	route("getSettings", call(func(t string, _ Empty) (any, error) {
		return svc.GetSettings(t)
	}))
	route("saveSettings", call(func(t string, in model.Settings) (any, error) {
		return Empty{}, svc.SaveSettings(t, in)
	}))
	route("recordPrint", call(func(t string, in model.PrintInput) (any, error) {
		return Empty{}, svc.RecordPrint(t, in)
	}))
	route("getLogo", call(func(t string, _ Empty) (any, error) {
		data, err := svc.GetLogo(t)
		return LogoBody{Data: data}, err
	}))
	route("saveLogo", call(func(t string, in LogoBody) (any, error) {
		return Empty{}, svc.SaveLogo(t, in.Data)
	}))
	return mux
}

func call[In any](fn func(token string, in In) (any, error)) func(string, []byte) (any, error) {
	return func(token string, body []byte) (any, error) {
		var in In
		if len(body) > 0 {
			if err := json.Unmarshal(body, &in); err != nil {
				return nil, model.Invalid("Request tidak valid")
			}
		}
		return fn(token, in)
	}
}

func respond(w http.ResponseWriter, out any, err error) {
	if err != nil {
		var ve *model.ValidationError
		switch {
		case errors.Is(err, model.ErrUnauthorized):
			writeError(w, http.StatusUnauthorized, ErrorBody{Code: "unauthorized", Error: err.Error()})
		case errors.Is(err, model.ErrForbidden):
			writeError(w, http.StatusForbidden, ErrorBody{Code: "forbidden", Error: err.Error()})
		case errors.Is(err, model.ErrNotFound):
			writeError(w, http.StatusNotFound, ErrorBody{Code: "not_found", Error: err.Error()})
		case errors.As(err, &ve):
			writeError(w, http.StatusBadRequest, ErrorBody{Code: "invalid", Error: ve.Message})
		default:
			writeError(w, http.StatusInternalServerError, ErrorBody{Code: "internal", Error: "Terjadi kesalahan di server"})
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func writeError(w http.ResponseWriter, status int, body ErrorBody) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
