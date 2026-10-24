package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const unitName = "fknat-ssh.service"

func UnitPath() string { return "/etc/systemd/system/" + unitName }

func InstallSystemd(configPath, executable string) error {
	if runtime.GOOS != "linux" {
		return errors.New("systemd installation is supported only on Linux")
	}
	if executable == "" {
		executable = "/usr/bin/fknat-ssh"
	}
	executableArg, err := systemdArg(executable)
	if err != nil {
		return err
	}
	configArg, err := systemdArg(configPath)
	if err != nil {
		return err
	}
	unit := fmt.Sprintf("[Unit]\nDescription=FKNAT SSH Transport\nAfter=network-online.target ssh.service\nWants=network-online.target\n\n[Service]\nType=simple\nUser=root\nExecStart=%s run --config %s\nRestart=always\nRestartSec=3\nKillSignal=SIGTERM\nTimeoutStopSec=15\nNoNewPrivileges=true\n\n[Install]\nWantedBy=multi-user.target\n", executableArg, configArg)
	if err := os.MkdirAll(filepath.Dir(UnitPath()), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(UnitPath(), []byte(unit), 0o644); err != nil {
		return err
	}
	if output, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command("systemctl", "enable", "--now", unitName).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable --now %s: %w: %s", unitName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func systemdArg(value string) (string, error) {
	if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("systemd argument is empty or contains a control character")
	}
	value = strings.ReplaceAll(value, "%", "%%")
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return "\"" + value + "\"", nil
}

func UninstallSystemd() error {
	if runtime.GOOS != "linux" {
		return errors.New("systemd installation is supported only on Linux")
	}
	_ = exec.Command("systemctl", "disable", "--now", unitName).Run()
	if err := os.Remove(UnitPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = exec.Command("systemctl", "daemon-reload").Run()
	return nil
}

func Control(action string) error {
	if runtime.GOOS != "linux" {
		return errors.New("service control is supported only on Linux")
	}
	return exec.Command("systemctl", action, unitName).Run()
}
