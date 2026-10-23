package updater

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"tiket/internal/version"
)

const (
	mirrorManifestPath = "/update/manifest"
	mirrorDownloadPath = "/update/download"
)

// Mirror lets the client PCs update from the server PC: it hands out the
// server's own exe together with the signed manifest of its release, so a
// client ends up on exactly the server's version.
type Mirror struct {
	Dir     string // where the manifest of the running version is kept
	Version string // version of the running app

	mu sync.Mutex
	m  *Manifest // nil until the running exe is known to match a release
}

func (s *Mirror) path() string { return filepath.Join(s.Dir, "manifest.json") }

// Save keeps the manifest of a version this PC is about to run, from the
// update it just installed.
func (s *Mirror) Save(m Manifest) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), body, 0o644)
}

// Ready reports whether clients can update from this PC.
func (s *Mirror) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m != nil
}

// Prepare finds the signed manifest of the running version: the one saved
// by the last update, otherwise the one of its release on the internet. It
// is only used when it matches the running exe, so a build that is not from
// a release is never handed out.
func (s *Mirror) Prepare(ctx context.Context) error {
	if s.Ready() {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	pub, err := publicKey()
	if err != nil {
		return err
	}
	var m Manifest
	body, err := os.ReadFile(s.path())
	if err == nil {
		err = json.Unmarshal(body, &m)
	}
	if err != nil || version.Normalize(m.Version) != version.Normalize(s.Version) || Verify(pub, data, m) != nil {
		if ReleasesURL == "" {
			return errors.New("build ini bukan dari rilis")
		}
		if m, err = fetchManifest(ctx, releaseURL(s.Version)); err != nil {
			return err
		}
		if err := Verify(pub, data, m); err != nil {
			return errors.New("file aplikasi ini tidak sama dengan rilis " + m.Version)
		}
		s.Save(m)
	}
	s.mu.Lock()
	s.m = &m
	s.mu.Unlock()
	return nil
}

// Handler serves the manifest and the exe. Without a matching release the
// manifest has the version only, so clients know to install by hand.
func (s *Mirror) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+mirrorManifestPath, func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		m := Manifest{Version: version.Normalize(s.Version)}
		if s.m != nil {
			m = *s.m
			m.URL = ""
		}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	})
	mux.HandleFunc("GET "+mirrorDownloadPath, func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			http.NotFound(w, r)
			return
		}
		exe, err := os.Executable()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, exe)
	})
	return mux
}
