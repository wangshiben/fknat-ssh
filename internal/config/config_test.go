package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadDefaultsAndPermissions(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.Mkdir(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	billing := filepath.Join(dir, "billing.json")
	if err := os.WriteFile(billing, []byte(`{"private_key_jwk":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	want := Config{BillingKeyFile: billing, DataDir: dataDir, IdentityFile: filepath.Join(dataDir, "server.identity"), SSHTarget: DefaultTarget, Protocol: "tcp", Route: 0}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SecurityProfile != "production" || got.Route != 0 || got.Protocol != "tcp" {
		t.Fatalf("config=%+v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions=%04o", info.Mode().Perm())
	}
}

func TestNormalizeRejectsNonZeroRoute(t *testing.T) {
	_, err := (Config{BillingKeyFile: "/tmp/key", DataDir: t.TempDir(), Route: 1}).Normalize()
	if err == nil {
		t.Fatal("route 1 was accepted")
	}
}

func TestNormalizeRejectsNonLoopbackTarget(t *testing.T) {
	_, err := (Config{BillingKeyFile: "/tmp/key", DataDir: t.TempDir(), SSHTarget: "192.168.1.10:22"}).Normalize()
	if err == nil {
		t.Fatal("non-loopback SSH target was accepted")
	}
}
