package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAndAddress(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	if err := os.Mkdir(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FKNAT_SSH_DATA_DIR", dataDir)
	billing := filepath.Join(dir, "billing.json")
	if err := os.WriteFile(billing, []byte(`{"private_key_jwk":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.json")
	var output bytes.Buffer
	if err := run([]string{"install", "--billing-key", billing, "--config", configPath, "--data-dir", dataDir, "--relay", "127.0.0.1:4101", "--no-service"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "installed successfully") {
		t.Fatalf("output=%s", output.String())
	}
	if !strings.Contains(output.String(), "Relay:        127.0.0.1:4101") {
		t.Fatalf("relay override missing from output=%s", output.String())
	}
	output.Reset()
	if err := run([]string{"address", "--identity", filepath.Join(dataDir, "server.identity")}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(output.String())) != 64 {
		t.Fatalf("address=%q", output.String())
	}
	output.Reset()
	if err := run([]string{"address", "--config", configPath}, &output, &output); err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(output.String())) != 64 {
		t.Fatalf("configured address=%q", output.String())
	}
}
