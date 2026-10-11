package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	ModeServer = "server"
	ModeClient = "client"

	DefaultPort = 8787
)

type Config struct {
	Mode        string `json:"mode"`
	Port        int    `json:"port"`
	ServerURL   string `json:"serverUrl"`
	Terminal    string `json:"terminal"`
	InvoiceCode string `json:"invoiceCode"`
	PrinterName string `json:"printerName"`
	PaperWidth  int    `json:"paperWidth"`
}

func DataDir() (string, error) {
	if dir := os.Getenv("TIKET_DATA_DIR"); dir != "" {
		return dir, os.MkdirAll(dir, 0o755)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Tiket")
	return dir, os.MkdirAll(dir, 0o755)
}

func path(dir string) string {
	return filepath.Join(dir, "config.json")
}

func Load(dir string) (Config, error) {
	cfg := Config{Port: DefaultPort, PaperWidth: 58}
	data, err := os.ReadFile(path(dir))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if cfg.PaperWidth != 80 {
		cfg.PaperWidth = 58
	}
	return cfg, nil
}

func Save(dir string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path(dir) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path(dir))
}
