package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"log"
	"os"
	"slices"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

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
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
