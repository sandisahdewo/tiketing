package config

import (
	"crypto/rand"
	"encoding/hex"
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
	Mode      string `json:"mode"`
	Port      int    `json:"port"`
	ServerURL string `json:"serverUrl"`
	// DeviceID identifies this PC to the server, which assigns it a counter.
	DeviceID string `json:"deviceId"`
	// Terminal is the name of this PC's counter as last seen, shown before
	// login.
	Terminal    string `json:"terminal"`
	PrinterName string `json:"printerName"`
	PaperWidth  int    `json:"paperWidth"`
}

// EnsureDeviceID gives the PC a permanent random ID the first time it runs.
func EnsureDeviceID(dir string, cfg *Config) error {
	if cfg.DeviceID != "" {
		return nil
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	cfg.DeviceID = hex.EncodeToString(b)
	return Save(dir, *cfg)
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
