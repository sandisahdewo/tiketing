// Command seed makes the database of the server the end-to-end tests run
// against, with its first admin (admin / rahasia). The first admin can only
// be made on the server PC itself, not over the network, so the tests cannot
// make it the way they make everything else.
//
//	go run ./e2e/seed <data folder of the server>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"tiket/internal/db"
	"tiket/internal/model"
	"tiket/internal/service"
	"tiket/internal/store"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: seed <data folder of the server>")
		os.Exit(2)
	}
	if err := seed(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func seed(dir string) error {
	conn, err := db.Open(filepath.Join(dir, "tiket.db"))
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Migrate(conn, filepath.Join(dir, "backups")); err != nil {
		return err
	}
	local := service.NewLocal(store.New(conn), "0.1.0")
	return local.CreateFirstAdmin(model.UserInput{Username: "admin", FullName: "Admin", Password: "rahasia"})
}
