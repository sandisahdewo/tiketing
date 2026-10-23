package updater

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"net/http/httptest"
	"os"
	"testing"
)

// A client follows the server: newer server versions come from the server
// PC, signed like a release, and versions it cannot work with are forced.
func TestCheckServer(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	defer func(k string) { publicKeyB64 = k }(publicKeyB64)
	publicKeyB64 = base64.StdEncoding.EncodeToString(pub)

	exe, _ := os.Executable()
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	sha, sig := Sign(priv, data)
	mirror := &Mirror{Dir: t.TempDir(), Version: "0.3.0"}
	ts := httptest.NewServer(mirror.Handler())
	defer ts.Close()
	ctx := context.Background()

	// Not from a release: clients learn the version but cannot get the file.
	info, _, err := CheckServer(ctx, ts.URL, "0.2.0")
	if err != nil || info.Available || !info.Manual {
		t.Fatalf("server without release file: %+v, %v", info, err)
	}
	if info, _, _ := CheckServer(ctx, ts.URL, "0.3.0"); info.Available || info.Manual || info.ServerBehind {
		t.Errorf("same version: %+v", info)
	}

	if err := mirror.Save(Manifest{Version: "v0.3.0", SHA256: sha, Signature: sig, Notes: "baru"}); err != nil {
		t.Fatal(err)
	}
	if err := mirror.Prepare(ctx); err != nil || !mirror.Ready() {
		t.Fatalf("prepare: %v", err)
	}

	info, m, err := CheckServer(ctx, ts.URL, "0.2.5")
	if err != nil || !info.Available || !info.Mandatory || !info.FromServer || info.Latest != "v0.3.0" || info.Notes != "baru" {
		t.Fatalf("older minor: %+v, %v", info, err)
	}
	if m.URL != ts.URL+mirrorDownloadPath {
		t.Errorf("download url %q", m.URL)
	}
	got, err := get(ctx, m.URL, maxDownload)
	if err != nil || Verify(pub, got, *m) != nil {
		t.Errorf("downloaded file does not verify: %v", err)
	}

	// A bug fix release of the same minor is offered but not forced.
	if info, _, _ := CheckServer(ctx, ts.URL, "0.3.0-rc1"); !info.Available || info.Mandatory {
		t.Errorf("older patch: %+v", info)
	}
	if info, _, _ := CheckServer(ctx, ts.URL, "0.3.2"); info.Available || info.ServerBehind {
		t.Errorf("newer patch on client: %+v", info)
	}
	if info, _, _ := CheckServer(ctx, ts.URL, "0.4.0"); info.Available || !info.ServerBehind {
		t.Errorf("newer minor on client: %+v", info)
	}
	if info, _, _ := CheckServer(ctx, ts.URL, "1.3.0"); !info.ServerBehind {
		t.Errorf("newer major on client: %+v", info)
	}
}

// A saved manifest that does not match the running exe is not handed out.
func TestMirrorRejectsOtherExe(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	defer func(k string) { publicKeyB64 = k }(publicKeyB64)
	publicKeyB64 = base64.StdEncoding.EncodeToString(pub)
	sha, sig := Sign(priv, []byte("exe lain"))
	mirror := &Mirror{Dir: t.TempDir(), Version: "0.3.0"}
	mirror.Save(Manifest{Version: "v0.3.0", SHA256: sha, Signature: sig})
	if err := mirror.Prepare(context.Background()); err == nil || mirror.Ready() {
		t.Errorf("other exe accepted: %v", err)
	}
}
