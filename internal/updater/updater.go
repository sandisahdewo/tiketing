package updater

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/minio/selfupdate"

	"tiket/internal/version"
)

// ReleasesURL is the releases page of the repository, such as
// https://github.com/owner/repo/releases. The release workflow sets it; a
// build without it never looks for updates on the internet.
var ReleasesURL = ""

// latestURL is the manifest of the newest release.
func latestURL() string { return ReleasesURL + "/latest/download/latest.json" }

// releaseURL is the manifest of release v.
func releaseURL(v string) string {
	return ReleasesURL + "/download/" + version.Normalize(v) + "/latest.json"
}

//go:embed publickey.txt
var publicKeyB64 string

const maxDownload = 300 << 20

type Manifest struct {
	Version   string `json:"version"`
	URL       string `json:"url"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
	Notes     string `json:"notes"`
	Mandatory bool   `json:"mandatory"`
}

type Info struct {
	Configured bool   `json:"configured"`
	Available  bool   `json:"available"`
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Notes      string `json:"notes"`
	Mandatory  bool   `json:"mandatory"`
	// Set on a client PC, which follows the server PC instead of the internet.
	FromServer bool `json:"fromServer"`
	// The server runs a version this PC cannot work with, but cannot hand
	// out its file (a build that is not from a release): install it by hand.
	Manual bool `json:"manual"`
	// The server runs an older version this PC cannot work with: the server
	// has to be updated first.
	ServerBehind bool `json:"serverBehind"`
}

func publicKey() (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKeyB64))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("public key update tidak valid")
	}
	return ed25519.PublicKey(raw), nil
}

func Configured() bool {
	_, err := publicKey()
	return ReleasesURL != "" && err == nil
}

var client = &http.Client{Timeout: 5 * time.Minute}

func get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tidak bisa terhubung ke server update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server update membalas status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("file update terlalu besar")
	}
	return data, nil
}

func Check(ctx context.Context, current string) (Info, *Manifest, error) {
	info := Info{Configured: Configured(), Current: current}
	if !info.Configured {
		return info, nil, nil
	}
	m, err := fetchManifest(ctx, latestURL())
	if err != nil {
		return info, nil, err
	}
	info.Latest = m.Version
	info.Notes = m.Notes
	info.Mandatory = m.Mandatory
	info.Available = version.Newer(m.Version, current)
	return info, &m, nil
}

func fetchManifest(ctx context.Context, url string) (Manifest, error) {
	var m Manifest
	data, err := get(ctx, url, 1<<20)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("manifest update tidak valid: %w", err)
	}
	if !version.Valid(m.Version) || !strings.HasPrefix(m.URL, "https://") {
		return m, errors.New("manifest update tidak valid")
	}
	return m, nil
}

// CheckServer is the update check of a client PC: it follows the version of
// the server at baseURL. The file comes from the server over the local
// network, signed like a release, so the client needs no internet.
func CheckServer(ctx context.Context, baseURL, current string) (Info, *Manifest, error) {
	_, keyErr := publicKey()
	info := Info{Configured: keyErr == nil, Current: current, FromServer: true}
	data, err := get(ctx, baseURL+mirrorManifestPath, 1<<20)
	if err != nil {
		return info, nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil || !version.Valid(m.Version) {
		return info, nil, errors.New("info update dari server tidak valid")
	}
	m.URL = baseURL + mirrorDownloadPath
	info.Latest, info.Notes = m.Version, m.Notes
	compatible := version.Compatible(m.Version, current)
	switch {
	case version.Newer(m.Version, current):
		info.Available = info.Configured && m.Signature != ""
		info.Manual = !info.Available && !compatible
		info.Mandatory = m.Mandatory || !compatible
	case !compatible:
		info.ServerBehind = true
	}
	return info, &m, nil
}

func Verify(pub ed25519.PublicKey, data []byte, m Manifest) error {
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), m.SHA256) {
		return errors.New("checksum file update tidak cocok")
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || !ed25519.Verify(pub, sum[:], sig) {
		return errors.New("tanda tangan file update tidak valid")
	}
	return nil
}

func Sign(priv ed25519.PrivateKey, data []byte) (sha string, signature string) {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), base64.StdEncoding.EncodeToString(ed25519.Sign(priv, sum[:]))
}

func DownloadAndApply(ctx context.Context, m Manifest) error {
	pub, err := publicKey()
	if err != nil {
		return err
	}
	data, err := get(ctx, m.URL, maxDownload)
	if err != nil {
		return err
	}
	if err := Verify(pub, data, m); err != nil {
		return err
	}
	if err := selfupdate.Apply(bytes.NewReader(data), selfupdate.Options{}); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update gagal dan rollback gagal: %w", rerr)
		}
		return fmt.Errorf("update gagal: %w", err)
	}
	return nil
}

func Restart(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Start()
}
