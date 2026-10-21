package applog

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tiket/internal/model"
)

func TestDailyFilesAndPrune(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"tiket-2026-09-20.log", "tiket-2026-09-21.log", "tiket-2026-10-01.log", "other.txt"} {
		os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644)
	}
	now := time.Date(2026, 10, 21, 23, 59, 0, 0, time.Local)
	logger, closer, err := open(dir, 30, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	defer closer.Close()

	logger.Info("CreateSale", "user", "budi")
	now = now.Add(2 * time.Minute)
	logger.Warn("DeleteSale", "error", "Nota sudah dihapus")

	var names []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		names = append(names, e.Name())
	}
	// 09-20 is pruned on open, 09-21 when the file rotates to 10-22.
	want := "other.txt tiket-2026-10-01.log tiket-2026-10-21.log tiket-2026-10-22.log"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("files = %s\nwant    %s", got, want)
	}
	day1, _ := os.ReadFile(filepath.Join(dir, "tiket-2026-10-21.log"))
	day2, _ := os.ReadFile(filepath.Join(dir, "tiket-2026-10-22.log"))
	if !strings.Contains(string(day1), "level=INFO msg=CreateSale user=budi") || strings.Contains(string(day1), "DeleteSale") {
		t.Errorf("day 1 log: %s", day1)
	}
	if !strings.Contains(string(day2), `level=WARN msg=DeleteSale error="Nota sudah dihapus"`) {
		t.Errorf("day 2 log: %s", day2)
	}
}

func TestLevel(t *testing.T) {
	cases := map[error]slog.Level{
		nil:                     slog.LevelInfo,
		model.Invalid("x"):      slog.LevelWarn,
		model.ErrForbidden:      slog.LevelWarn,
		model.ErrUnauthorized:   slog.LevelWarn,
		errors.New("disk full"): slog.LevelError,
	}
	for err, want := range cases {
		if got := Level(err); got != want {
			t.Errorf("Level(%v) = %v, want %v", err, got, want)
		}
	}
}

func TestPayloadRedacts(t *testing.T) {
	got := Payload(map[string]any{
		"username": "budi",
		"Password": "rahasia",
		"data":     "iVBORw0KGgo",
		"items":    []any{map[string]any{"token": "abc", "qty": 2}},
		"empty":    map[string]any{"password": ""},
	})
	want := `{"Password":"***","data":"***","empty":{"password":""},"items":[{"qty":2,"token":"***"}],"username":"budi"}`
	if got != want {
		t.Errorf("Payload = %s\nwant      %s", got, want)
	}
	if got := PayloadJSON([]byte("not json")); got != "not json" {
		t.Errorf("non-JSON body = %s", got)
	}
	if got := PayloadJSON([]byte(`"` + strings.Repeat("x", 3000) + `"`)); len(got) > maxPayload+len("…") {
		t.Errorf("payload not truncated: %d", len(got))
	}
}

func TestErrorHasStack(t *testing.T) {
	var buf strings.Builder
	prev := slog.Default()
	defer slog.SetDefault(prev)
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	Action("Ok", nil)
	Action("Invalid", model.Invalid("x"))
	Action("Broken", errors.New("disk full"))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 || strings.Contains(lines[0], "stack=") || strings.Contains(lines[1], "stack=") {
		t.Fatalf("only ERROR lines should carry a stack:\n%s", buf.String())
	}
	if !strings.Contains(lines[2], "applog_test.go:") || !strings.Contains(lines[2], "TestErrorHasStack") {
		t.Errorf("stack should start at the caller: %s", lines[2])
	}
}
