package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"tiket/internal/model"
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
	Search  string `json:"search"`
	Deleted bool   `json:"deleted"`
}

type Empty struct{}

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
			clientVersion := r.Header.Get(HeaderVersion)
			if !version.Compatible(clientVersion, serverVersion) {
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
					writeError(w, http.StatusBadRequest, ErrorBody{Code: "invalid", Error: "Request tidak valid"})
					return
				}
				body = raw
			}
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			out, err := fn(token, body)
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
		return svc.ListTickets(t, in.Search, in.Deleted)
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
	route("report", call(func(t string, in model.ReportQuery) (any, error) {
		return svc.Report(t, in)
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
			log.Printf("server error: %v", err)
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
