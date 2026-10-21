// Package applog writes the application log to one file per day in the data
// directory and removes files older than KeepDays.
package applog

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"tiket/internal/model"
)

const (
	KeepDays   = 30
	filePrefix = "tiket-"
	fileSuffix = ".log"
	dayLayout  = "2006-01-02"

	maxPayload    = 2000
	maxStackDepth = 12
)

// sensitive JSON keys are masked in logged payloads (logo data is just large).
var sensitive = map[string]bool{"password": true, "token": true, "authorization": true, "data": true}

var (
	ctxMu   sync.Mutex
	base    *slog.Logger
	version string
)

// Init makes logger the default and tags every line with the app version.
func Init(logger *slog.Logger, appVersion string) {
	ctxMu.Lock()
	defer ctxMu.Unlock()
	base, version = logger, appVersion
	slog.SetDefault(base.With("v", version))
}

// SetTerminal tags every following line with this PC's counter name.
func SetTerminal(terminal string) {
	ctxMu.Lock()
	defer ctxMu.Unlock()
	if base == nil {
		return
	}
	slog.SetDefault(base.With("v", version, "loket", terminal))
}

// NewOpID returns a short random id for one user action (one click). Every
// step of that action logs it, on the client and on the server.
func NewOpID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// SessionID identifies one login in the log without revealing the token. The
// client and the server derive the same value from the same token.
func SessionID(token string) string {
	if token == "" {
		return "-"
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:4])
}

// Payload renders v as JSON for the log with sensitive fields masked.
func Payload(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return PayloadJSON(b)
}

// PayloadJSON masks sensitive fields in a raw JSON body and shortens it.
func PayloadJSON(body []byte) string {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return truncate(string(body))
	}
	redact(v)
	out, _ := json.Marshal(v)
	return truncate(string(out))
}

func redact(v any) {
	switch v := v.(type) {
	case map[string]any:
		for k, val := range v {
			if s, ok := val.(string); ok && sensitive[strings.ToLower(k)] {
				if s != "" {
					v[k] = "***"
				}
				continue
			}
			redact(val)
		}
	case []any:
		for _, val := range v {
			redact(val)
		}
	}
}

func truncate(s string) string {
	if len(s) <= maxPayload {
		return s
	}
	return s[:maxPayload] + "…"
}

// stack lists the callers of the logging function as "file:line function",
// newest first, without runtime and logging frames.
func stack() string {
	pcs := make([]uintptr, 32)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	var out []string
	for {
		f, more := frames.Next()
		fn := f.Function[strings.LastIndex(f.Function, "/")+1:]
		if !strings.HasPrefix(fn, "runtime.") && fn != "applog.Action" && fn != "applog.stack" {
			out = append(out, fmt.Sprintf("%s:%d %s", filepath.Base(f.File), f.Line, fn))
		}
		if !more || len(out) == maxStackDepth {
			break
		}
	}
	return strings.Join(out, " < ")
}

type dailyFile struct {
	mu   sync.Mutex
	dir  string
	keep int
	now  func() time.Time
	day  string
	f    *os.File
}

func (d *dailyFile) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if day := d.now().Format(dayLayout); day != d.day || d.f == nil {
		if err := d.rotate(day); err != nil {
			return 0, err
		}
	}
	return d.f.Write(p)
}

func (d *dailyFile) rotate(day string) error {
	if d.f != nil {
		d.f.Close()
		d.f = nil
	}
	f, err := os.OpenFile(filepath.Join(d.dir, filePrefix+day+fileSuffix), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	d.f, d.day = f, day
	d.prune()
	return nil
}

// prune removes log files whose date is more than keep days before today.
func (d *dailyFile) prune() {
	entries, err := os.ReadDir(d.dir)
	if err != nil {
		return
	}
	cutoff := d.now().AddDate(0, 0, -d.keep).Format(dayLayout)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, fileSuffix) {
			continue
		}
		day := strings.TrimSuffix(strings.TrimPrefix(name, filePrefix), fileSuffix)
		if _, err := time.Parse(dayLayout, day); err == nil && day < cutoff {
			os.Remove(filepath.Join(d.dir, name))
		}
	}
}

func (d *dailyFile) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.f == nil {
		return nil
	}
	err := d.f.Close()
	d.f = nil
	return err
}

// Dir is the log folder inside the data folder.
func Dir(dataDir string) string {
	return filepath.Join(dataDir, "logs")
}

// Zip writes every daily log file in dir to w as a zip archive and returns
// how many it added.
func Zip(dir string, w io.Writer) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	zw := zip.NewWriter(w)
	n := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, fileSuffix) {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return n, err
		}
		f, err := zw.Create(name)
		if err != nil {
			return n, err
		}
		if _, err := f.Write(body); err != nil {
			return n, err
		}
		n++
	}
	return n, zw.Close()
}

// Open creates dir and returns a logger writing to today's file in it.
func Open(dir string) (*slog.Logger, io.Closer, error) {
	return open(dir, KeepDays, time.Now)
}

func open(dir string, keep int, now func() time.Time) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	w := &dailyFile{dir: dir, keep: keep, now: now}
	if err := w.rotate(now().Format(dayLayout)); err != nil {
		return nil, nil, err
	}
	h := slog.NewTextHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey && len(groups) == 0 {
				return slog.String(slog.TimeKey, a.Value.Time().Format("2006-01-02 15:04:05"))
			}
			return a
		},
	})
	return slog.New(h), w, nil
}

// Level is INFO for success, WARN for errors caused by the user (invalid
// input, no access, expired session, version mismatch) and ERROR otherwise.
func Level(err error) slog.Level {
	var ve *model.ValidationError
	switch {
	case err == nil:
		return slog.LevelInfo
	case errors.As(err, &ve),
		errors.Is(err, model.ErrForbidden),
		errors.Is(err, model.ErrNotFound),
		errors.Is(err, model.ErrUnauthorized),
		errors.Is(err, model.ErrVersionMismatch):
		return slog.LevelWarn
	}
	return slog.LevelError
}

// Action logs one action with its outcome to the default logger. ERROR lines
// also carry a stack trace.
func Action(action string, err error, attrs ...any) {
	level := Level(err)
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	if level == slog.LevelError {
		attrs = append(attrs, "stack", stack())
	}
	slog.Default().Log(context.Background(), level, action, attrs...)
}
