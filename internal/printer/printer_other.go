//go:build !windows

package printer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const FileTarget = "Simpan ke file (uji coba)"

func List(_ string) ([]string, error) {
	return []string{FileTarget}, nil
}

func Default() string {
	return FileTarget
}

func Print(_ string, data []byte, dataDir string) (string, error) {
	dir := filepath.Join(dataDir, "print-out")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("nota-%s.bin", time.Now().Format("20060102-150405.000")))
	return path, os.WriteFile(path, data, 0o644)
}
