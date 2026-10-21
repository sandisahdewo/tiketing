package service

import (
	"tiket/internal/model"
)

type Service interface {
	Status() (model.Status, error)
	CreateFirstAdmin(in model.UserInput) error
	Login(username, password string) (model.Session, error)
	Logout(token string) error
	Me(token string) (model.User, error)

	ListTickets(token, search string, deleted, activeOnly bool) ([]model.Ticket, error)
	SaveTicket(token string, in model.TicketInput) (model.Ticket, error)
	DeleteTicket(token string, id int64) error
	RestoreTicket(token string, id int64) error
	SetTicketActive(token string, id int64, active bool) error
	SearchTickets(token string, q model.TicketQuery) (model.TicketPage, error)

	ListUsers(token string) ([]model.User, error)
	SaveUser(token string, in model.UserInput) (model.User, error)
	DeleteUser(token string, id int64) error

	CreateSale(token string, in model.SaleInput) (model.Sale, error)
	GetSale(token string, id int64) (model.Sale, error)
	ListSales(token string, q model.SaleQuery) ([]model.Sale, error)
	SearchSales(token string, q model.SalePageQuery) (model.SalePage, error)
	DeleteSale(token string, in model.DeleteSaleInput) error
	Report(token string, q model.ReportQuery) (model.Report, error)

	RecordPrint(token string, in model.PrintInput) error

	GetSettings(token string) (model.Settings, error)
	SaveSettings(token string, in model.Settings) error
	GetLogo(token string) (string, error)
	SaveLogo(token, base64Data string) error
}
