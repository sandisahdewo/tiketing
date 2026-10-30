package service

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"tiket/internal/model"
	"tiket/internal/store"
)

// maxCash keeps a cash amount sane (Rp 1 trillion, in sen).
const maxCash = 1_000_000_000_000 * 100

const maxShiftName = 30

// validateShiftType checks a shift type and writes its hours as HH:MM.
func validateShiftType(in *model.ShiftTypeInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return model.Invalid("Nama shift wajib diisi")
	}
	if utf8.RuneCountInString(in.Name) > maxShiftName {
		return model.Invalid("Nama shift maksimal 30 karakter")
	}
	start, err := time.Parse("15:04", strings.TrimSpace(in.StartTime))
	if err != nil {
		return model.Invalid("Jam mulai tidak valid, contoh 06:00")
	}
	end, err := time.Parse("15:04", strings.TrimSpace(in.EndTime))
	if err != nil {
		return model.Invalid("Jam selesai tidak valid, contoh 14:00")
	}
	in.StartTime, in.EndTime = start.Format("15:04"), end.Format("15:04")
	if in.StartTime == in.EndTime {
		return model.Invalid("Jam selesai tidak boleh sama dengan jam mulai")
	}
	return nil
}

func (l *Local) ListShiftTypes(token string) ([]model.ShiftType, error) {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return nil, err
	}
	return l.Store.ListShiftTypes(false)
}

func (l *Local) SaveShiftType(token string, in model.ShiftTypeInput) (model.ShiftType, error) {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return model.ShiftType{}, err
	}
	if err := validateShiftType(&in); err != nil {
		return model.ShiftType{}, err
	}
	if in.ID == 0 {
		return l.Store.CreateShiftType(in)
	}
	return l.Store.UpdateShiftType(in)
}

func (l *Local) SetShiftTypeActive(token string, id int64, active bool) error {
	if _, err := l.user(token, model.RoleAdmin); err != nil {
		return err
	}
	return l.Store.SetShiftTypeActive(id, active)
}

// ShiftStatus tells the PC deviceID whether its user can sell: their open
// shift (at any counter), or else whether another user's shift holds this
// PC's counter, and which shift to suggest opening.
func (l *Local) ShiftStatus(token, deviceID string) (model.ShiftStatus, error) {
	u, err := l.user(token)
	if err != nil {
		return model.ShiftStatus{}, err
	}
	now := l.Store.Now()
	out := model.ShiftStatus{ServerTime: now.Format("2006-01-02 15:04:05")}
	c, err := l.Store.CounterByDevice(strings.TrimSpace(deviceID))
	switch {
	case err == nil:
		out.CounterID, out.CounterName = c.ID, c.Name
	case !errors.Is(err, model.ErrNotFound):
		return out, err
	}
	own, err := l.Store.OpenShiftOf(u.ID)
	if err != nil {
		return out, err
	}
	if own != nil {
		r, err := l.Store.ShiftReport(own.ID)
		if err != nil {
			return out, err
		}
		out.Own = &r
	}
	if out.CounterID > 0 {
		held, err := l.Store.OpenShiftAt(out.CounterID)
		if err != nil {
			return out, err
		}
		if held != nil && held.UserID != u.ID {
			out.Blocking = held
		}
	}
	if out.Types, err = l.Store.ListShiftTypes(true); err != nil {
		return out, err
	}
	out.SuggestedTypeID = store.SuggestShiftType(out.Types, now)
	return out, nil
}

func (l *Local) OpenShift(token string, in model.OpenShiftInput) (model.Shift, error) {
	u, err := l.user(token)
	if err != nil {
		return model.Shift{}, err
	}
	if in.OpeningCash < 0 || in.OpeningCash > maxCash {
		return model.Shift{}, model.Invalid("Modal awal tidak valid")
	}
	return l.Store.OpenShift(u.ID, strings.TrimSpace(in.DeviceID), in.TypeID, in.OpeningCash)
}

// shiftAccess gets a shift the user may see, close and correct: any for an
// admin; for a cashier their own while it is open and on the day it closed.
func (l *Local) shiftAccess(token string, id int64) (model.User, error) {
	u, err := l.user(token)
	if err != nil {
		return u, err
	}
	h, err := l.Store.GetShift(id)
	if err != nil {
		return u, err
	}
	if u.Role == model.RoleAdmin {
		return u, nil
	}
	if h.UserID != u.ID || (h.ClosedAt != nil && !strings.HasPrefix(*h.ClosedAt, l.today())) {
		return u, model.ErrForbidden
	}
	return u, nil
}

func (l *Local) CloseShift(token string, in model.CloseShiftInput) (model.ShiftClosing, error) {
	u, err := l.shiftAccess(token, in.ShiftID)
	if err != nil {
		return model.ShiftClosing{}, err
	}
	if in.ActualCash < 0 || in.ActualCash > maxCash {
		return model.ShiftClosing{}, model.Invalid("Kas aktual tidak valid")
	}
	in.Note = strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(in.Note) > 200 {
		return model.ShiftClosing{}, model.Invalid("Keterangan maksimal 200 karakter")
	}
	return l.Store.CloseShift(in, u.ID)
}

func (l *Local) GetShift(token string, id int64) (model.ShiftReport, error) {
	if _, err := l.shiftAccess(token, id); err != nil {
		return model.ShiftReport{}, err
	}
	return l.Store.ShiftReport(id)
}

// ShiftDay lists the shifts of a business date with the ones still open from
// other dates (admin). A cashier gets their own shifts that are open or were
// closed today, whatever the date asked.
func (l *Local) ShiftDay(token string, q model.ShiftDayQuery) (model.ShiftDay, error) {
	u, err := l.user(token)
	if err != nil {
		return model.ShiftDay{}, err
	}
	if u.Role != model.RoleAdmin {
		today := l.today()
		shifts, err := l.Store.UserShifts(u.ID, today)
		return model.ShiftDay{Date: today, Shifts: shifts, Unclosed: []model.Shift{}}, err
	}
	if _, err := time.Parse("2006-01-02", q.Date); err != nil {
		return model.ShiftDay{}, model.Invalid("Tanggal tidak valid")
	}
	shifts, err := l.Store.ShiftsOfDay(q.Date)
	if err != nil {
		return model.ShiftDay{}, err
	}
	unclosed, err := l.Store.UnclosedShifts(q.Date)
	return model.ShiftDay{Date: q.Date, Shifts: shifts, Unclosed: unclosed}, err
}

func (l *Local) RecordShiftClosingPrint(token string, closingID int64) error {
	if _, err := l.user(token); err != nil {
		return err
	}
	c, err := l.Store.GetShiftClosing(closingID)
	if err != nil {
		return err
	}
	if _, err := l.shiftAccess(token, c.Shift.ID); err != nil {
		return err
	}
	return l.Store.RecordShiftClosingPrint(closingID)
}
