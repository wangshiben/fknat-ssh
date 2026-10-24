package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const DefaultTarget = "127.0.0.1:22"

type Config struct {
	BillingKeyFile      string `json:"billing_key_file"`
	IdentityFile        string `json:"identity_file"`
	BillingSnapshotFile string `json:"billing_snapshot_file,omitempty"`
	SecurityProfile     string `json:"security_profile"`
	IndexAddress        string `json:"index_address,omitempty"`
	RelayAddress        string `json:"relay_address,omitempty"`
	SSHTarget           string `json:"ssh_target"`
	Route               uint16 `json:"route"`
	Protocol            string `json:"protocol"`
	DataDir             string `json:"data_dir"`
}

func DefaultDataDir() string {
	if value := strings.TrimSpace(os.Getenv("FKNAT_SSH_DATA_DIR")); value != "" {
		return value
	}
	switch runtime.GOOS {
	case "windows":
		if value := os.Getenv("ProgramData"); value != "" {
			return filepath.Join(value, "fknat-ssh")
		}
	case "darwin":
		return "/Library/Application Support/fknat-ssh"
	default:
		return "/var/lib/fknat-ssh"
	}
	return filepath.Join(os.TempDir(), "fknat-ssh")
}

func DefaultConfigFile() string {
	if value := strings.TrimSpace(os.Getenv("FKNAT_SSH_CONFIG")); value != "" {
		return value
	}
	return filepath.Join(DefaultDataDir(), "config.json")
}

func Defaults() Config {
	dataDir := DefaultDataDir()
	return Config{IdentityFile: filepath.Join(dataDir, "server.identity"), BillingSnapshotFile: filepath.Join(dataDir, "server.identity.billing.json"), SecurityProfile: "production", SSHTarget: DefaultTarget, Route: 0, Protocol: "tcp", DataDir: dataDir}
}

func (cfg Config) DataDirOrDefault() string {
	if strings.TrimSpace(cfg.DataDir) != "" {
		return cfg.DataDir
	}
	return DefaultDataDir()
}

func (cfg Config) Normalize() (Config, error) {
	defaults := Defaults()
	if cfg.SecurityProfile == "" {
		cfg.SecurityProfile = defaults.SecurityProfile
	}
	if cfg.SSHTarget == "" {
		cfg.SSHTarget = defaults.SSHTarget
	}
	if cfg.DataDir == "" {
		cfg.DataDir = cfg.DataDirOrDefault()
	}
	if absolute, err := filepath.Abs(cfg.DataDir); err == nil {
		cfg.DataDir = absolute
	} else {
		return Config{}, err
	}
	if cfg.IdentityFile == "" {
		cfg.IdentityFile = filepath.Join(cfg.DataDir, "server.identity")
	}
	if cfg.BillingSnapshotFile == "" {
		cfg.BillingSnapshotFile = cfg.IdentityFile + ".billing.json"
	} else if absolute, err := filepath.Abs(cfg.BillingSnapshotFile); err == nil {
		cfg.BillingSnapshotFile = absolute
	} else {
		return Config{}, err
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "tcp"
	}
	cfg.SecurityProfile = strings.ToLower(strings.TrimSpace(cfg.SecurityProfile))
	if cfg.SecurityProfile != "production" {
		return Config{}, errors.New("fknat-ssh: security profile must be production")
	}
	if strings.TrimSpace(cfg.BillingKeyFile) == "" {
		return Config{}, errors.New("fknat-ssh: billing key file is required")
	}
	if absolute, err := filepath.Abs(cfg.BillingKeyFile); err == nil {
		cfg.BillingKeyFile = absolute
	} else {
		return Config{}, err
	}
	if absolute, err := filepath.Abs(cfg.IdentityFile); err == nil {
		cfg.IdentityFile = absolute
	} else {
		return Config{}, err
	}
	if cfg.Protocol != "tcp" || cfg.Route != 0 {
		return Config{}, errors.New("fknat-ssh: only TCP route 0 is supported")
	}
	if host, portText, err := net.SplitHostPort(cfg.SSHTarget); err != nil || strings.TrimSpace(host) == "" {
		return Config{}, fmt.Errorf("fknat-ssh: invalid SSH target %q", cfg.SSHTarget)
	} else if port, parseErr := strconv.Atoi(portText); parseErr != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("fknat-ssh: invalid SSH target port %q", portText)
	} else if ip := net.ParseIP(host); !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
		return Config{}, errors.New("fknat-ssh: SSH target must use localhost or a loopback IP")
	}
	return cfg, nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("fknat-ssh: parse config: %w", err)
	}
	return cfg.Normalize()
}

func Save(path string, cfg Config) error {
	cfg, err := cfg.Normalize()
	if err != nil {
		return err
	}
	if err := ensurePrivateDirectory(cfg.DataDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(path, 0o700)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("fknat-ssh: data directory path is not a directory")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return readErr
		}
		if len(entries) != 0 && filepath.Base(filepath.Clean(path)) != "fknat-ssh" {
			return fmt.Errorf("fknat-ssh: refusing to change permissions on non-empty shared directory %s", path)
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return fmt.Errorf("fknat-ssh: protect data directory: %w", err)
		}
	}
	return nil
}

func ValidateBillingKey(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 64<<10 {
		return errors.New("billing key must be a regular file between 1 and 65536 bytes")
	}
	return nil
}
