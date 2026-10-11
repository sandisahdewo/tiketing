package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

const keepBackups = 14

func Open(path string) (*sql.DB, error) {
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func migrations() ([]string, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func Migrate(conn *sql.DB, backupDir string) error {
	var current int
	if err := conn.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	names, err := migrations()
	if err != nil {
		return err
	}
	if current >= len(names) {
		return nil
	}
	if current > 0 {
		if _, err := Backup(conn, backupDir, fmt.Sprintf("pre-migrate-v%d", current)); err != nil {
			return fmt.Errorf("backup sebelum migrasi gagal: %w", err)
		}
	}
	for i := current; i < len(names); i++ {
		body, err := migrationFiles.ReadFile("migrations/" + names[i])
		if err != nil {
			return err
		}
		tx, err := conn.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrasi %s: %w", names[i], err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func Backup(conn *sql.DB, dir, label string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("tiket-%s-%s.db", time.Now().Format("20060102-150405"), label)
	target := filepath.Join(dir, name)
	if _, err := conn.Exec("VACUUM INTO ?", target); err != nil {
		return "", err
	}
	return target, prune(dir)
}

func BackupDaily(conn *sql.DB, dir string) error {
	today := "tiket-" + time.Now().Format("20060102")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), today) && strings.HasSuffix(e.Name(), "-harian.db") {
			return nil
		}
	}
	_, err := Backup(conn, dir, "harian")
	return err
}

func prune(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "tiket-") && strings.HasSuffix(e.Name(), ".db") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for len(names) > keepBackups {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}
