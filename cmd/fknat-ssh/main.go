package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wangshiben/fknat-ssh/internal/config"
	"github.com/wangshiben/fknat-ssh/internal/health"
	"github.com/wangshiben/fknat-ssh/internal/service"
	"github.com/wangshiben/fknat-ssh/internal/status"
	core "github.com/wangshiben/fknat/pkg/node"
	"github.com/wangshiben/fknat/pkg/server"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "fknat-ssh: %v\n", err)
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "billing key"):
		return 11
	case strings.Contains(message, "identity"):
		return 12
	case strings.Contains(message, "ssh backend"):
		return 13
	case strings.Contains(message, "discov"):
		return 15
	case strings.Contains(message, "permission"):
		return 19
	default:
		return 18
	}
}

func run(args []string, output, errorOutput io.Writer) error {
	if len(args) == 0 {
		return errors.New("command required; use --help")
	}
	switch args[0] {
	case "install":
		return install(args[1:], output)
	case "uninstall":
		return service.UninstallSystemd()
	case "run":
		return runDaemon(args[1:], output, errorOutput)
	case "start", "stop", "restart":
		return service.Control(args[0])
	case "status":
		return showStatus(args[1:], output)
	case "address":
		return showAddress(args[1:], output)
	case "doctor":
		return doctor(args[1:], output)
	case "version":
		_, _ = fmt.Fprintf(output, "fknat-ssh %s\n", version)
		return nil
	case "--help", "-h", "help":
		printUsage(output)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage(output io.Writer) {
	_, _ = fmt.Fprintln(output, "fknat-ssh install|uninstall|run|start|stop|restart|status|address|doctor|version")
}

func install(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(output)
	billingKey := flags.String("billing-key", "", "CA Web billing private key JSON path")
	sshTarget := flags.String("ssh-target", config.DefaultTarget, "local SSH backend host:port")
	indexAddress := flags.String("index", "", "override the FKNAT index address")
	relayAddress := flags.String("relay", "", "override the FKNAT relay address")
	configPath := flags.String("config", config.DefaultConfigFile(), "configuration file")
	dataDir := flags.String("data-dir", config.DefaultDataDir(), "persistent data directory")
	noService := flags.Bool("no-service", false, "write configuration without installing systemd")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*billingKey) == "" {
		return errors.New("--billing-key is required")
	}
	if err := config.ValidateBillingKey(*billingKey); err != nil {
		return fmt.Errorf("billing key: %w", err)
	}
	cfg := config.Config{BillingKeyFile: *billingKey, IdentityFile: filepath.Join(*dataDir, "server.identity"), BillingSnapshotFile: filepath.Join(*dataDir, "server.identity.billing.json"), SecurityProfile: "production", IndexAddress: *indexAddress, RelayAddress: *relayAddress, SSHTarget: *sshTarget, Protocol: "tcp", Route: 0, DataDir: *dataDir}
	if err := config.Save(*configPath, cfg); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	identity, err := core.Prepare(context.Background(), core.CommonConfig{BillingKeyFile: cfg.BillingKeyFile, IdentityFile: cfg.IdentityFile, BillingSnapshotFile: cfg.BillingSnapshotFile, SecurityProfile: "production"})
	if err != nil {
		return err
	}
	_ = status.Write(filepath.Join(cfg.DataDir, "status.json"), status.Snapshot{State: "stopped", Address: identity.NodeID, Backend: cfg.SSHTarget, BackendStatus: "unknown", NetworkStatus: "offline"})
	banner, bannerErr := health.CheckSSHBanner(context.Background(), cfg.SSHTarget)
	if bannerErr != nil {
		_, _ = fmt.Fprintf(output, "Warning: SSH backend unhealthy: %v\n", bannerErr)
	} else {
		_, _ = fmt.Fprintf(output, "SSH backend: healthy (%s)\n", banner)
	}
	if !*noService {
		if runtime.GOOS != "linux" {
			return errors.New("service installation is currently supported only on Linux")
		}
		if err := service.InstallSystemd(*configPath, executablePath()); err != nil {
			return fmt.Errorf("service install: %w", err)
		}
	}
	_, _ = fmt.Fprintln(output, "fknat-SSH installed successfully.")
	installedStatus := "starting"
	if *noService {
		installedStatus = "stopped"
	}
	_, _ = fmt.Fprintf(output, "Status:       %s\n", installedStatus)
	_, _ = fmt.Fprintf(output, "Address:      %s\n", identity.NodeID)
	_, _ = fmt.Fprintf(output, "SSH backend:  %s\n", cfg.SSHTarget)
	_, _ = fmt.Fprintln(output, "Route:        0")
	networkDescription := "auto"
	if cfg.IndexAddress != "" {
		networkDescription = "index " + cfg.IndexAddress
	}
	relayDescription := "auto"
	if cfg.RelayAddress != "" {
		networkDescription = "explicit relay"
		relayDescription = cfg.RelayAddress
	}
	_, _ = fmt.Fprintf(output, "Network:      %s\n", networkDescription)
	_, _ = fmt.Fprintf(output, "Relay:        %s\n", relayDescription)
	return nil
}

func runDaemon(args []string, output, errorOutput io.Writer) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	flags.SetOutput(output)
	configPath := flags.String("config", config.DefaultConfigFile(), "configuration file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	identity, err := core.Prepare(context.Background(), core.CommonConfig{BillingKeyFile: cfg.BillingKeyFile, IdentityFile: cfg.IdentityFile, BillingSnapshotFile: cfg.BillingSnapshotFile, SecurityProfile: cfg.SecurityProfile})
	if err != nil {
		return err
	}
	backendStatus := "unhealthy"
	if _, healthErr := health.CheckSSHBanner(context.Background(), cfg.SSHTarget); healthErr == nil {
		backendStatus = "healthy"
	} else {
		fmt.Fprintf(errorOutput, "level=warn event=ssh_backend_unhealthy err=%v\n", healthErr)
	}
	statusPath := filepath.Join(cfg.DataDir, "status.json")
	_ = status.Write(statusPath, status.Snapshot{State: "starting", Address: identity.NodeID, Backend: cfg.SSHTarget, BackendStatus: backendStatus, NetworkStatus: "connecting"})
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals()...)
	defer stop()
	runtimeServer, err := server.New(server.Config{Common: core.CommonConfig{BillingKeyFile: cfg.BillingKeyFile, IdentityFile: cfg.IdentityFile, BillingSnapshotFile: cfg.BillingSnapshotFile, SecurityProfile: "production"}, Routes: []server.Route{{Protocol: "tcp", Key: 0, Target: cfg.SSHTarget}}, IndexAddress: cfg.IndexAddress, RelayAddress: cfg.RelayAddress})
	if err != nil {
		_ = status.Write(statusPath, status.Snapshot{State: "fatal", Address: identity.NodeID, Backend: cfg.SSHTarget, BackendStatus: backendStatus, NetworkStatus: "offline", LastError: err.Error()})
		return err
	}
	defer runtimeServer.Close()
	go func() {
		<-ctx.Done()
		_ = status.Write(statusPath, status.Snapshot{State: "stopping", Address: identity.NodeID, Backend: cfg.SSHTarget, BackendStatus: backendStatus, NetworkStatus: "offline"})
		_ = runtimeServer.Close()
	}()
	_, _ = fmt.Fprintf(output, "Address: %s\n", identity.NodeID)
	runDone := make(chan error, 1)
	go func() { runDone <- runtimeServer.Start(ctx) }()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err = <-runDone:
			goto runtimeDone
		case <-ctx.Done():
			err = <-runDone
			goto runtimeDone
		case <-ticker.C:
			current := runtimeServer.Status()
			_ = status.Write(statusPath, status.Snapshot{
				State: string(current.State), Address: identity.NodeID, Backend: cfg.SSHTarget,
				BackendStatus: backendStatus, NetworkStatus: current.NetworkStatus,
				RelayCount: current.RelayCount, ActiveSessions: current.ActiveSessions,
				ActiveStreams: current.ActiveStreams, AcceptedTotal: current.AcceptedTotal,
				RejectedTotal: current.RejectedTotal, StartedAt: current.StartedAt,
				LastError: current.LastError,
			})
		}
	}
runtimeDone:
	if err != nil {
		_ = status.Write(statusPath, status.Snapshot{State: "fatal", Address: identity.NodeID, Backend: cfg.SSHTarget, BackendStatus: backendStatus, NetworkStatus: "offline", LastError: err.Error()})
		return err
	}
	_ = status.Write(statusPath, status.Snapshot{State: "stopped", Address: identity.NodeID, Backend: cfg.SSHTarget, BackendStatus: backendStatus, NetworkStatus: "offline"})
	return nil
}

func showAddress(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("address", flag.ContinueOnError)
	flags.SetOutput(output)
	configPath := flags.String("config", config.DefaultConfigFile(), "configuration file")
	identityPath := flags.String("identity", "", "identity file override")
	if err := flags.Parse(args); err != nil {
		return err
	}
	selectedIdentity := strings.TrimSpace(*identityPath)
	if selectedIdentity == "" {
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		selectedIdentity = cfg.IdentityFile
	}
	address, err := core.ReadNodeID(selectedIdentity)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(output, address)
	return nil
}

func showStatus(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(output)
	configPath := flags.String("config", config.DefaultConfigFile(), "configuration file")
	jsonOutput := flags.Bool("json", false, "JSON output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	snapshot, err := status.Read(filepath.Join(cfg.DataDir, "status.json"))
	if err != nil {
		return err
	}
	if *jsonOutput {
		data, _ := json.MarshalIndent(snapshot, "", "  ")
		_, _ = fmt.Fprintln(output, string(data))
		return nil
	}
	_, _ = fmt.Fprintf(output, "Status:       %s\nAddress:      %s\nBackend:      %s\nSSH:           %s\nNetwork:       %s\nRelay:        %d\nSessions:     %d\nStreams:      %d\n", snapshot.State, snapshot.Address, snapshot.Backend, snapshot.BackendStatus, snapshot.NetworkStatus, snapshot.RelayCount, snapshot.ActiveSessions, snapshot.ActiveStreams)
	if snapshot.LastError != "" {
		_, _ = fmt.Fprintf(output, "Last error:    %s\n", snapshot.LastError)
	}
	return nil
}

func doctor(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(output)
	configPath := flags.String("config", config.DefaultConfigFile(), "configuration file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	checksOK := true
	check := func(name string, checkErr error) {
		if checkErr != nil {
			checksOK = false
			_, _ = fmt.Fprintf(output, "[FAIL] %s: %v\n", name, checkErr)
		} else {
			_, _ = fmt.Fprintf(output, "[OK] %s\n", name)
		}
	}
	check("billing key file readable", config.ValidateBillingKey(cfg.BillingKeyFile))
	identity, identityErr := core.Prepare(context.Background(), core.CommonConfig{BillingKeyFile: cfg.BillingKeyFile, IdentityFile: cfg.IdentityFile, BillingSnapshotFile: cfg.BillingSnapshotFile, SecurityProfile: "production"})
	check("identity writable", identityErr)
	_, bannerErr := health.CheckSSHBanner(context.Background(), cfg.SSHTarget)
	check("ssh banner valid", bannerErr)
	if cfg.IndexAddress == "" && cfg.RelayAddress == "" {
		_, discoveryErr := core.DiscoverIndex(context.Background(), core.ProductionCAURL)
		check("Index discovery", discoveryErr)
	} else {
		check("Index/Relay override", nil)
	}
	if identityErr == nil {
		_, _ = fmt.Fprintf(output, "Address: %s\n", identity.NodeID)
	}
	if !checksOK {
		return errors.New("doctor found one or more failures")
	}
	return nil
}

func executablePath() string {
	path, err := os.Executable()
	if err != nil {
		return "/usr/bin/fknat-ssh"
	}
	return path
}
