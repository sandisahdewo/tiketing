package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"tiket/internal/updater"
	"tiket/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(os.Args[2:])
	case "manifest":
		err = manifest(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `pemakaian:
  tiket-release keygen [-public internal/updater/publickey.txt]
  tiket-release manifest -file tiket.exe -version v1.0.0 -url https://... [-notes "..."] [-mandatory] [-out latest.json]
    (kunci privat dibaca dari env TIKET_SIGNING_KEY)`)
	os.Exit(2)
}

func keygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	publicPath := fs.String("public", "internal/updater/publickey.txt", "lokasi file public key")
	fs.Parse(args)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*publicPath, []byte(base64.StdEncoding.EncodeToString(pub)+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Public key ditulis ke %s (commit file ini).\n", *publicPath)
	fmt.Fprintln(os.Stderr, "Kunci privat di bawah ini: simpan sebagai GitHub secret TIKET_SIGNING_KEY. JANGAN di-commit.")
	fmt.Println(base64.StdEncoding.EncodeToString(priv))
	return nil
}

func manifest(args []string) error {
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	file := fs.String("file", "", "file exe yang dirilis")
	ver := fs.String("version", "", "versi rilis, contoh v1.0.0")
	url := fs.String("url", "", "URL unduh exe (https)")
	notes := fs.String("notes", "", "catatan rilis")
	mandatory := fs.Bool("mandatory", false, "update wajib")
	out := fs.String("out", "latest.json", "file manifest keluaran")
	fs.Parse(args)

	if *file == "" || !version.Valid(*ver) || !strings.HasPrefix(*url, "https://") {
		usage()
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("TIKET_SIGNING_KEY")))
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return fmt.Errorf("TIKET_SIGNING_KEY kosong atau tidak valid")
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	sha, sig := updater.Sign(ed25519.PrivateKey(raw), data)
	m := updater.Manifest{
		Version:   version.Normalize(*ver),
		URL:       *url,
		SHA256:    sha,
		Signature: sig,
		Notes:     *notes,
		Mandatory: *mandatory,
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(*out, append(body, '\n'), 0o644)
}
