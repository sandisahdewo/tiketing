package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"tiket/internal/applog"
	"tiket/internal/config"
)

//go:embed all:frontend/dist
var assets embed.FS

var appVersion = "0.1.0"

const afterUpdateFlag = "--after-update"

func main() {
	if slices.Contains(os.Args[1:], afterUpdateFlag) {
		time.Sleep(2 * time.Second)
	}

	dataDir, err := config.DataDir()
	if err != nil {
		log.Fatal(err)
	}
	if logger, closer, err := applog.Open(filepath.Join(dataDir, "logs")); err == nil {
		// Also routes the standard log package into the file.
		applog.Init(logger, appVersion)
		defer closer.Close()
	} else {
		log.Printf("log file tidak bisa dibuka: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Crash", "panic", r, "stack", string(debug.Stack()))
			panic(r)
		}
	}()
	slog.Info("AppStart", "dataDir", dataDir, "args", os.Args[1:])

	app := NewApp(dataDir)
	lockID := sha256.Sum256([]byte(dataDir))

	err = wails.Run(&options.App{
		Title:     "Tiket",
		Width:     1280,
		Height:    800,
		MinWidth:  1024,
		MinHeight: 680,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 241, G: 245, B: 249, A: 1},
		WindowStartState: options.Maximised,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "tiket-" + hex.EncodeToString(lockID[:8]),
		},
		Linux: &linux.Options{
			WebviewGpuPolicy: linuxGpuPolicy(),
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		slog.Error("AppExit", "error", err.Error())
		log.Fatal(err)
	}
	slog.Info("AppExit")
}

// linuxGpuPolicy lets the webview use the GPU when it helps. Without Linux
// options Wails turns hardware acceleration off (a workaround for some
// drivers, wails#2977), which makes long lists such as the cashier screen
// scroll slowly. TIKET_GPU=never turns it off again on a machine that shows
// a blank or broken window.
func linuxGpuPolicy() linux.WebviewGpuPolicy {
	switch os.Getenv("TIKET_GPU") {
	case "never":
		return linux.WebviewGpuPolicyNever
	case "always":
		return linux.WebviewGpuPolicyAlways
	}
	return linux.WebviewGpuPolicyOnDemand
}
