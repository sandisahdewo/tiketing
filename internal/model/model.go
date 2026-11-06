package model

import (
	"errors"
	"fmt"
	"time"
)

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
	Active    bool    `json:"active"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	DeletedAt *string `json:"deletedAt"`
}

const (
	TicketAll      = "all"
	TicketActive   = "active"
	TicketInactive = "inactive"
	TicketDeleted  = "deleted"
)

// TicketQuery asks for one page of tickets. Status is one of the Ticket*
// constants (empty means all non-deleted); Category is empty for both.
// Sort is a column key (empty: tickets before topups, then by name) and Dir
// is "asc" or "desc".
type TicketQuery struct {
	Search   string `json:"search"`
	Category string `json:"category"`
	Status   string `json:"status"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Sort     string `json:"sort"`
	Dir      string `json:"dir"`
}

// TicketPage is one page of tickets plus the number of all matches.
type TicketPage struct {
	Tickets  []Ticket `json:"tickets"`
	Count    int64    `json:"count"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
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

// SaleInput is a sale from the PC with DeviceID; the server takes the counter
// (name and invoice code) from that PC's assignment. With ReplacesSaleID it
// corrects that sale, which is deleted with Reason in the same step.
type SaleInput struct {
	DeviceID       string          `json:"deviceId"`
	Items          []SaleItemInput `json:"items"`
	Paid           int64           `json:"paid"`
	ReplacesSaleID int64           `json:"replacesSaleId"`
	Reason         string          `json:"reason"`
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
	ID          int64  `json:"id"`
	InvoiceNo   string `json:"invoiceNo"`
	UserID      int64  `json:"userId"`
	CashierName string `json:"cashierName"`
	// Terminal is the counter name printed on the receipt when the sale was
	// made; CounterName is that counter's name now (for lists and reports).
	Terminal    string `json:"terminal"`
	CounterID   int64  `json:"counterId"`
	CounterName string `json:"counterName"`
	// ShiftName is the shift the sale was made in ("" before shifts).
	ShiftName    string     `json:"shiftName"`
	Total        int64      `json:"total"`
	Paid         int64      `json:"paid"`
	ChangeAmount int64      `json:"changeAmount"`
	CreatedAt    string     `json:"createdAt"`
	PrintCount   int64      `json:"printCount"`
	Items        []SaleItem `json:"items"`

	DeletedAt     *string `json:"deletedAt"`
	DeletedByName string  `json:"deletedByName"`
	DeleteReason  string  `json:"deleteReason"`

	// ShiftID is 0 for sales from before shifts.
	ShiftID int64 `json:"shiftId"`
	// A correction names the sale it replaces; a corrected (deleted) sale
	// names the one that replaced it.
	ReplacesSaleID      int64  `json:"replacesSaleId"`
	ReplacesInvoiceNo   string `json:"replacesInvoiceNo"`
	ReplacedByInvoiceNo string `json:"replacedByInvoiceNo"`
}

type SaleQuery struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Deleted bool   `json:"deleted"`
	Search  string `json:"search"`
	ReportFilter
}

// ReportFilter narrows a report or sale list; zero values mean "all".
// Category keeps the sales that contain at least one item of it (and, in a
// report, counts only those items).
type ReportFilter struct {
	Category    string `json:"category"`
	UserID      int64  `json:"userId"`
	CounterID   int64  `json:"counterId"`
	ShiftTypeID int64  `json:"shiftTypeId"`
}

// ReportFilters lists the choices for the report filters.
type ReportFilters struct {
	Cashiers   []ReportCashier `json:"cashiers"`
	Counters   []Counter       `json:"counters"`
	ShiftTypes []ShiftType     `json:"shiftTypes"`
}

type ReportCashier struct {
	ID       int64  `json:"id"`
	FullName string `json:"fullName"`
	Deleted  bool   `json:"deleted"`
}

// SalePageQuery asks for one page of sales; Page starts at 1. Sort is one of
// the SaleSort* keys (empty: newest first) and Dir is "asc" or "desc".
type SalePageQuery struct {
	SaleQuery
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Sort     string `json:"sort"`
	Dir      string `json:"dir"`
}

const (
	SortAsc  = "asc"
	SortDesc = "desc"
)

// SalePage is one page of sales plus the count and amount of every match.
type SalePage struct {
	Sales    []Sale `json:"sales"`
	Count    int64  `json:"count"`
	Amount   int64  `json:"amount"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type DeleteSaleInput struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

type ReportQuery struct {
	Period string `json:"period"`
	From   string `json:"from"`
	To     string `json:"to"`
	ReportFilter
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

var (
	dayNames   = []string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	monthNames = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
)

// PeriodLabel is a report period as people read it, for a report of the given
// kind ("daily", "weekly" or "monthly"), e.g. "Senin, 05-10-2026".
func PeriodLabel(period, kind string) string {
	if kind == "monthly" {
		t, err := time.Parse("2006-01", period)
		if err != nil {
			return period
		}
		return fmt.Sprintf("%s %d", monthNames[t.Month()-1], t.Year())
	}
	t, err := time.Parse("2006-01-02", period)
	if err != nil {
		return period
	}
	if kind == "weekly" {
		return t.Format("02-01-2006") + " - " + t.AddDate(0, 0, 6).Format("02-01-2006")
	}
	return dayNames[t.Weekday()] + ", " + t.Format("02-01-2006")
}

// ItemGroup is the items of one category of a report with their subtotal.
type ItemGroup struct {
	Label string
	Items []ReportTicketRow
	Qty   int64
	Total int64
}

// ItemGroups groups the report's items by category, leaving out empty ones.
func (r Report) ItemGroups() []ItemGroup {
	categories := []struct{ value, label string }{
		{CategoryTicket, "Tiket"},
		{CategoryTopup, "Topup Card"},
	}
	var groups []ItemGroup
	for _, c := range categories {
		g := ItemGroup{Label: c.label}
		for _, it := range r.Tickets {
			if it.Category == c.value {
				g.Items = append(g.Items, it)
				g.Qty += it.Qty
				g.Total += it.Total
			}
		}
		if len(g.Items) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
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

// ShiftType is a shift an admin sets up, e.g. Siang 06:00-14:00. An EndTime
// at or before StartTime ends the next day.
type ShiftType struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type ShiftTypeInput struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// Shift is one cashier's shift at one counter. Name and hours are those of
// its type when it opened. BusinessDate is the day it belongs to (a night
// shift opened after midnight belongs to the day before) and ScheduledEndAt
// is when it should end. ClosedAt is nil while it is open.
type Shift struct {
	ID             int64   `json:"id"`
	TypeID         int64   `json:"typeId"`
	Name           string  `json:"name"`
	StartTime      string  `json:"startTime"`
	EndTime        string  `json:"endTime"`
	BusinessDate   string  `json:"businessDate"`
	ScheduledEndAt string  `json:"scheduledEndAt"`
	UserID         int64   `json:"userId"`
	CashierName    string  `json:"cashierName"`
	CounterID      int64   `json:"counterId"`
	CounterName    string  `json:"counterName"`
	OpeningCash    int64   `json:"openingCash"`
	OpenedAt       string  `json:"openedAt"`
	ClosedAt       *string `json:"closedAt"`
}

// ShiftSales counts the sales of a shift.
type ShiftSales struct {
	Transactions int64  `json:"transactions"`
	TicketQty    int64  `json:"ticketQty"`
	TicketTotal  int64  `json:"ticketTotal"`
	TopupQty     int64  `json:"topupQty"`
	TopupTotal   int64  `json:"topupTotal"`
	Total        int64  `json:"total"`
	FirstSaleAt  string `json:"firstSaleAt"`
	LastSaleAt   string `json:"lastSaleAt"`
	DeletedCount int64  `json:"deletedCount"`
	DeletedTotal int64  `json:"deletedTotal"`
}

// ShiftClosing is the cash counted when a shift closed, kept with the shift's
// sales as they were then, so it reads the same later on. A correction is a
// new closing with the next revision.
type ShiftClosing struct {
	ID       int64      `json:"id"`
	Shift    Shift      `json:"shift"`
	Revision int64      `json:"revision"`
	Sales    ShiftSales `json:"sales"`
	// ExpectedCash is the opening cash plus the sales.
	ExpectedCash int64  `json:"expectedCash"`
	ActualCash   int64  `json:"actualCash"`
	Difference   int64  `json:"difference"`
	Note         string `json:"note"`
	ClosedByName string `json:"closedByName"`
	ClosedAt     string `json:"closedAt"`
	PrintCount   int64  `json:"printCount"`
	// The actual cash of the revision before, for a correction.
	PreviousActual *int64 `json:"previousActual"`
	// The sales of the shift deleted by the time of this closing.
	Deleted []DeletedSale `json:"deleted"`
}

// DeletedSale is a deleted sale of a shift, for reviewing the shift: deleted
// outright or replaced by a correction (ReplacedByInvoiceNo).
type DeletedSale struct {
	ID                  int64  `json:"id"`
	InvoiceNo           string `json:"invoiceNo"`
	Total               int64  `json:"total"`
	CreatedAt           string `json:"createdAt"`
	DeletedAt           string `json:"deletedAt"`
	DeletedByName       string `json:"deletedByName"`
	Reason              string `json:"reason"`
	ReplacedByInvoiceNo string `json:"replacedByInvoiceNo"`
}

// ShiftReport is a shift with its sales now and its closings.
type ShiftReport struct {
	Shift        Shift      `json:"shift"`
	Sales        ShiftSales `json:"sales"`
	ExpectedCash int64      `json:"expectedCash"`
	// Deleted lists the shift's deleted sales, oldest first.
	Deleted []DeletedSale `json:"deleted"`
	// The closing that counts (the latest revision), nil while open.
	Closing *ShiftClosing  `json:"closing"`
	History []ShiftClosing `json:"history"`
	// The sales changed after the closing, e.g. a sale deleted later.
	Changed bool `json:"changed"`
}

// ShiftStatus is what the cashier screen of a PC needs: the user's open
// shift (Own, at any counter), another user's shift holding this PC's
// counter (Blocking, without its numbers) and the types a shift can be
// opened with. ServerTime lets the PC judge ScheduledEndAt by the server's
// clock.
type ShiftStatus struct {
	CounterID       int64        `json:"counterId"`
	CounterName     string       `json:"counterName"`
	Own             *ShiftReport `json:"own"`
	Blocking        *Shift       `json:"blocking"`
	Types           []ShiftType  `json:"types"`
	SuggestedTypeID int64        `json:"suggestedTypeId"`
	ServerTime      string       `json:"serverTime"`
}

// OpenShiftInput opens a shift at the counter of the PC DeviceID, with
// OpeningCash in the drawer.
type OpenShiftInput struct {
	DeviceID    string `json:"deviceId"`
	TypeID      int64  `json:"typeId"`
	OpeningCash int64  `json:"openingCash"`
}

// CloseShiftInput closes a shift (Revision 0) or corrects its closing
// (Revision is the latest one + 1). A stale Revision is refused, so a shift
// closed meanwhile from another PC is not corrected by accident.
type CloseShiftInput struct {
	ShiftID    int64  `json:"shiftId"`
	Revision   int64  `json:"revision"`
	ActualCash int64  `json:"actualCash"`
	Note       string `json:"note"`
}

type ShiftDayQuery struct {
	Date string `json:"date"`
}

// ShiftDay lists the shifts of a business date, and the open shifts of other
// dates so a forgotten one is found.
type ShiftDay struct {
	Date     string        `json:"date"`
	Shifts   []ShiftReport `json:"shifts"`
	Unclosed []Shift       `json:"unclosed"`
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

// PrintInput records a print of a sale on the PC with DeviceID.
type PrintInput struct {
	SaleID   int64  `json:"saleId"`
	DeviceID string `json:"deviceId"`
}

// Counter is a loket: a named point of sale with its own invoice code, used
// by at most one PC (DeviceID, empty when free).
type Counter struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	InvoiceCode string  `json:"invoiceCode"`
	Active      bool    `json:"active"`
	DeviceID    string  `json:"deviceId"`
	DeviceName  string  `json:"deviceName"`
	LastSeenAt  *string `json:"lastSeenAt"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

type CounterInput struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	InvoiceCode string `json:"invoiceCode"`
}

// AssignCounterInput puts the PC DeviceID on a counter (CounterID 0 frees the
// PC). A counter used by another PC is only taken over with Force.
type AssignCounterInput struct {
	CounterID  int64  `json:"counterId"`
	DeviceID   string `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	Force      bool   `json:"force"`
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
