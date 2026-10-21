package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"tiket/internal/model"
)

const timeLayout = "2006-01-02 15:04:05"

type Store struct {
	DB  *sql.DB
	Now func() time.Time
}

func New(conn *sql.DB) *Store {
	return &Store{DB: conn, Now: time.Now}
}

func (s *Store) now() string {
	return s.Now().Format(timeLayout)
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrNotFound
	}
	return err
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func likePattern(search string) string {
	return "%" + likeEscaper.Replace(strings.TrimSpace(search)) + "%"
}

// orderBy builds an ORDER BY clause from a whitelisted sort key, falling back
// to def when key is empty. The id column breaks ties so paging stays stable.
func orderBy(columns map[string]string, key, dir, def, id string) (string, error) {
	if key == "" {
		return " ORDER BY " + def, nil
	}
	expr, ok := columns[key]
	if !ok {
		return "", model.Invalid("Kolom urutan tidak valid")
	}
	switch dir {
	case "", model.SortAsc:
		dir = "ASC"
	case model.SortDesc:
		dir = "DESC"
	default:
		return "", model.Invalid("Arah urutan tidak valid")
	}
	return " ORDER BY " + expr + " " + dir + ", " + id + " " + dir, nil
}

func likePrefix(prefix string) string {
	return likeEscaper.Replace(prefix) + "%"
}
