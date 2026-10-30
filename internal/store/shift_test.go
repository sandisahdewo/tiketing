package store

import (
	"testing"
	"time"

	"tiket/internal/model"
)

func at(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

func TestShiftWindow(t *testing.T) {
	const layout = "2006-01-02 15:04"
	for _, tc := range []struct {
		start, end, now, date, startAt, endAt string
	}{
		{"06:00", "14:00", "2026-10-05 09:00", "2026-10-05", "2026-10-05 06:00", "2026-10-05 14:00"},
		{"06:00", "14:00", "2026-10-05 05:00", "2026-10-05", "2026-10-05 06:00", "2026-10-05 14:00"}, // early
		{"06:00", "14:00", "2026-10-05 15:00", "2026-10-05", "2026-10-05 06:00", "2026-10-05 14:00"}, // late
		{"22:00", "06:00", "2026-10-05 23:00", "2026-10-05", "2026-10-05 22:00", "2026-10-06 06:00"},
		{"22:00", "06:00", "2026-10-06 01:30", "2026-10-05", "2026-10-05 22:00", "2026-10-06 06:00"}, // after midnight
		{"22:00", "06:00", "2026-10-06 06:00", "2026-10-06", "2026-10-06 22:00", "2026-10-07 06:00"}, // the night is over
		{"22:00", "00:00", "2026-10-05 23:00", "2026-10-05", "2026-10-05 22:00", "2026-10-06 00:00"},
		{"22:00", "06:00", "2026-11-01 02:00", "2026-10-31", "2026-10-31 22:00", "2026-11-01 06:00"}, // across a month
	} {
		date, startAt, endAt, err := ShiftWindow(tc.start, tc.end, at(t, tc.now))
		if err != nil || date != tc.date || startAt.Format(layout) != tc.startAt || endAt.Format(layout) != tc.endAt {
			t.Errorf("%s-%s at %s: %s %s %s %v; want %s %s %s", tc.start, tc.end, tc.now,
				date, startAt.Format(layout), endAt.Format(layout), err, tc.date, tc.startAt, tc.endAt)
		}
	}
	for _, bad := range [][2]string{{"06:00", "06:00"}, {"25:00", "06:00"}, {"06:00", ""}} {
		if _, _, _, err := ShiftWindow(bad[0], bad[1], at(t, "2026-10-05 09:00")); err == nil {
			t.Errorf("%s-%s accepted", bad[0], bad[1])
		}
	}
}

func TestSuggestShiftType(t *testing.T) {
	three := []model.ShiftType{
		{ID: 1, StartTime: "06:00", EndTime: "14:00"},
		{ID: 2, StartTime: "14:00", EndTime: "22:00"},
		{ID: 3, StartTime: "22:00", EndTime: "06:00"},
	}
	day := three[:2]
	overlap := append([]model.ShiftType{{ID: 4, StartTime: "10:00", EndTime: "12:00"}}, three...)
	for _, tc := range []struct {
		types []model.ShiftType
		now   string
		want  int64
	}{
		{three, "2026-10-05 05:59", 3},
		{three, "2026-10-05 06:00", 1},
		{three, "2026-10-05 13:59", 1},
		{three, "2026-10-05 14:00", 2},
		{three, "2026-10-06 03:00", 3},
		{day, "2026-10-05 05:00", 1}, // none running: the next to start
		{day, "2026-10-05 23:00", 1}, // tomorrow's
		{overlap, "2026-10-05 11:00", 4},
		{overlap, "2026-10-05 12:00", 1},
		{nil, "2026-10-05 11:00", 0},
	} {
		if got := SuggestShiftType(tc.types, at(t, tc.now)); got != tc.want {
			t.Errorf("%d types at %s: got %d, want %d", len(tc.types), tc.now, got, tc.want)
		}
	}
}
