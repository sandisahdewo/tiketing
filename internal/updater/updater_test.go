package updater

import (
	"crypto/ed25519"
	"testing"

	"tiket/internal/version"
)

func TestVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("isi exe baru")
	sha, sig := Sign(priv, data)
	m := Manifest{SHA256: sha, Signature: sig}

	if err := Verify(pub, data, m); err != nil {
		t.Fatalf("valid update rejected: %v", err)
	}
	if err := Verify(pub, []byte("isi exe palsu"), m); err == nil {
		t.Fatal("tampered data accepted")
	}

	otherPub, otherPriv, _ := ed25519.GenerateKey(nil)
	_, badSig := Sign(otherPriv, data)
	if err := Verify(pub, data, Manifest{SHA256: sha, Signature: badSig}); err == nil {
		t.Fatal("signature from another key accepted")
	}
	if err := Verify(otherPub, data, m); err == nil {
		t.Fatal("wrong public key accepted")
	}
}

func TestVersions(t *testing.T) {
	if !version.Newer("1.3.0", "v1.2.9") {
		t.Error("1.3.0 should be newer than 1.2.9")
	}
	if version.Newer("1.2.0", "1.2.0") {
		t.Error("same version is not newer")
	}
	if !version.Compatible("1.2.0", "v1.2.7") {
		t.Error("same major.minor should be compatible")
	}
	if version.Compatible("1.3.0", "1.2.7") {
		t.Error("different minor should be incompatible")
	}
	if version.Compatible("", "1.2.0") {
		t.Error("empty version should be incompatible")
	}
}
