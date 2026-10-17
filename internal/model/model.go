package model

import "errors"

const (
	RoleAdmin = "admin"
	RoleKasir = "kasir"
)

const (
	CategoryTicket = "ticket"
	CategoryTopup  = "topup"
	DefaultColor   = "orange"
)

var TicketColors = []string{"orange", "yellow", "red", "rose", "pink", "purple", "indigo", "blue", "sky", "teal", "emerald", "lime", "slate"}

var (
	ErrUnauthorized    = errors.New("Sesi berakhir, silakan login kembali")
	ErrForbidden       = errors.New("Anda tidak punya akses untuk fitur ini")
	ErrNotFound        = errors.New("Data tidak ditemukan")
	ErrVersionMismatch = errors.New("Versi aplikasi tidak cocok dengan server")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func Invalid(msg string) error {
	return &ValidationError{Message: msg}
}

type Ticket struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Price     int64   `json:"price"`
	Category  string  `json:"category"`
	Color     string  `json:"color"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	DeletedAt *string `json:"deletedAt"`
}

type TicketInput struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Category string `json:"category"`
	Color    string `json:"color"`
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FullName  string `json:"fullName"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type UserInput struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	FullName string `json:"fullName"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

type Session struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type SaleItemInput struct {
	TicketID int64 `json:"ticketId"`
	Qty      int64 `json:"qty"`
}

type SaleInput struct {
	Terminal    string          `json:"terminal"`
	InvoiceCode string          `json:"invoiceCode"`
	Items       []SaleItemInput `json:"items"`
	Paid        int64           `json:"paid"`
}

type SaleItem struct {
	ID         int64  `json:"id"`
	TicketID   int64  `json:"ticketId"`
	TicketName string `json:"ticketName"`
	Category   string `json:"category"`
	Price      int64  `json:"price"`
	Qty        int64  `json:"qty"`
	Subtotal   int64  `json:"subtotal"`
}

type Sale struct {
	ID           int64      `json:"id"`
	InvoiceNo    string     `json:"invoiceNo"`
	UserID       int64      `json:"userId"`
	CashierName  string     `json:"cashierName"`
	Terminal     string     `json:"terminal"`
	Total        int64      `json:"total"`
	Paid         int64      `json:"paid"`
	ChangeAmount int64      `json:"changeAmount"`
	CreatedAt    string     `json:"createdAt"`
	PrintCount   int64      `json:"printCount"`
	Items        []SaleItem `json:"items"`
}

type SaleQuery struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ReportQuery struct {
	Period string `json:"period"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type ReportRow struct {
	Period       string `json:"period"`
	Transactions int64  `json:"transactions"`
	Qty          int64  `json:"qty"`
	Total        int64  `json:"total"`
	TicketQty    int64  `json:"ticketQty"`
	TicketTotal  int64  `json:"ticketTotal"`
	TopupQty     int64  `json:"topupQty"`
	TopupTotal   int64  `json:"topupTotal"`
}

type ReportTicketRow struct {
	TicketID   int64  `json:"ticketId"`
	TicketName string `json:"ticketName"`
	Category   string `json:"category"`
	Qty        int64  `json:"qty"`
	Total      int64  `json:"total"`
}

type Report struct {
	Period       string            `json:"period"`
	From         string            `json:"from"`
	To           string            `json:"to"`
	Rows         []ReportRow       `json:"rows"`
	Tickets      []ReportTicketRow `json:"tickets"`
	Transactions int64             `json:"transactions"`
	Qty          int64             `json:"qty"`
	Total        int64             `json:"total"`
	TicketQty    int64             `json:"ticketQty"`
	TicketTotal  int64             `json:"ticketTotal"`
	TopupQty     int64             `json:"topupQty"`
	TopupTotal   int64             `json:"topupTotal"`
}

type Settings struct {
	BusinessName   string `json:"businessName"`
	Address        string `json:"address"`
	Footer         string `json:"footer"`
	InvoicePrefix  string `json:"invoicePrefix"`
	PurposeWeekday string `json:"purposeWeekday"`
	PurposeWeekend string `json:"purposeWeekend"`
	HasLogo        bool   `json:"hasLogo"`
}

const (
	DefaultInvoicePrefix  = "TKT"
	DefaultPurposeWeekday = "WEEKDAY REGULER"
	DefaultPurposeWeekend = "WEEKEND REGULER"
)

type PrintInput struct {
	SaleID   int64  `json:"saleId"`
	Terminal string `json:"terminal"`
}

type Status struct {
	Version  string `json:"version"`
	HasUsers bool   `json:"hasUsers"`
}

type VersionMismatchError struct {
	Message string
}

func (e *VersionMismatchError) Error() string {
	return e.Message
}

func (e *VersionMismatchError) Is(target error) bool {
	return target == ErrVersionMismatch
}
