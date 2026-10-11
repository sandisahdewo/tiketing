package store

import (
	"database/sql"

	"tiket/internal/model"
)

const (
	keyBusinessName   = "business_name"
	keyAddress        = "address"
	keyFooter         = "footer"
	keyInvoicePrefix  = "invoice_prefix"
	keyPurposeWeekday = "purpose_weekday"
	keyPurposeWeekend = "purpose_weekend"
	keyLogo           = "logo"
)

type querier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func settingValue(q querier, key, fallback string) (string, error) {
	var v string
	err := q.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	if err == sql.ErrNoRows || (err == nil && v == "") {
		return fallback, nil
	}
	return v, err
}

func (s *Store) GetSettings() (model.Settings, error) {
	rows, err := s.DB.Query("SELECT key, value FROM settings WHERE key <> ?", keyLogo)
	if err != nil {
		return model.Settings{}, err
	}
	defer rows.Close()
	values := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return model.Settings{}, err
		}
		values[k] = v
	}
	if err := rows.Err(); err != nil {
		return model.Settings{}, err
	}
	or := func(key, fallback string) string {
		if values[key] == "" {
			return fallback
		}
		return values[key]
	}
	var hasLogo bool
	err = s.DB.QueryRow("SELECT EXISTS (SELECT 1 FROM settings WHERE key = ? AND value <> '')", keyLogo).Scan(&hasLogo)
	return model.Settings{
		BusinessName:   values[keyBusinessName],
		Address:        values[keyAddress],
		Footer:         values[keyFooter],
		InvoicePrefix:  or(keyInvoicePrefix, model.DefaultInvoicePrefix),
		PurposeWeekday: or(keyPurposeWeekday, model.DefaultPurposeWeekday),
		PurposeWeekend: or(keyPurposeWeekend, model.DefaultPurposeWeekend),
		HasLogo:        hasLogo,
	}, err
}

func (s *Store) SaveSettings(st model.Settings) error {
	return s.setValues(map[string]string{
		keyBusinessName:   st.BusinessName,
		keyAddress:        st.Address,
		keyFooter:         st.Footer,
		keyInvoicePrefix:  st.InvoicePrefix,
		keyPurposeWeekday: st.PurposeWeekday,
		keyPurposeWeekend: st.PurposeWeekend,
	})
}

func (s *Store) GetLogo() (string, error) {
	return settingValue(s.DB, keyLogo, "")
}

func (s *Store) SaveLogo(base64Data string) error {
	return s.setValues(map[string]string{keyLogo: base64Data})
}

func (s *Store) setValues(values map[string]string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range values {
		_, err := tx.Exec(
			"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value",
			k, v,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
