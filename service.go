package urso

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

const (
	// DefaultUrsoServiceName is the default name for the urso system service.
	DefaultUrsoServiceName = "com.urso-run.urso"
)

// ServiceConfig defines the parameters for installing a system service.
type ServiceConfig struct {
	Name           string
	ExecutablePath string
	Arguments      []string
	UrsoHome       string
}

// ServiceManager defines the interface for managing system services across different OSs.
type ServiceManager interface {
	// Install configures and enables a system service based on the provided config.
	Install(ctx context.Context, cfg ServiceConfig) error

	// Uninstall stops and removes a system service by name.
	Uninstall(ctx context.Context, serviceName string) error
}

// ErrUnsupportedOS is returned when an operation is attempted on an unsupported operating system.
var ErrUnsupportedOS = errors.New("unsupported operating system: only macOS and Linux are supported")

// NewServiceManager creates a new ServiceManager appropriate for the current OS.
func NewServiceManager(logger *slog.Logger, ursoHome string) (ServiceManager, error) {
	switch runtime.GOOS {
	case "darwin":
		return newLaunchdManager(logger, ursoHome), nil
	case "linux":
		return newSystemdManager(logger, ursoHome), nil
	default:
		return nil, ErrUnsupportedOS
	}
}

const launchdTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Name}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.ExecutablePath}}</string>
        {{range .Arguments}}<string>{{.}}</string>
        {{end}}
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ThrottleInterval</key>
    <integer>60</integer>
    <key>StandardOutPath</key>
    <string>{{.UrsoHome}}/logs/{{.Name}}.log</string>
    <key>StandardErrorPath</key>
    <string>{{.UrsoHome}}/logs/{{.Name}}.log</string>
</dict>
</plist>
`

const systemdTemplate = `[Unit]
Description={{.Name}}
After=network.target

[Service]
ExecStart={{.ExecutablePath}} {{.ArgumentsJoined}}
Restart=always
RestartSec=60
StandardOutput=append:{{.UrsoHome}}/logs/{{.Name}}.log
StandardError=append:{{.UrsoHome}}/logs/{{.Name}}.log

[Install]
WantedBy=default.target
`

// LaunchdManager implements the ServiceManager interface for macOS systems using launchd.
type LaunchdManager struct {
	logger   *slog.Logger
	ursoHome string
}

func newLaunchdManager(logger *slog.Logger, ursoHome string) *LaunchdManager {
	return &LaunchdManager{logger: logger, ursoHome: ursoHome}
}

func (l *LaunchdManager) Install(ctx context.Context, cfg ServiceConfig) error {
	l.logger.Info("installing launchd user agent", "service", cfg.Name)

	plistPath, err := l.getPlistPath(cfg.Name)
	if err != nil {
		return err
	}

	if err := l.createPlistFile(cfg, plistPath); err != nil {
		return err
	}

	uid := os.Getuid()
	domain := fmt.Sprintf("gui/%d", uid)

	// Bootout first in case it's already bootstrapped
	if err := l.runLaunchctl(ctx, "bootout", domain, plistPath); err != nil {
		l.logger.Debug("failed to bootout existing service (expected if not installed)", "error", err)
	}

	if err := l.runLaunchctl(ctx, "bootstrap", domain, plistPath); err != nil {
		return fmt.Errorf("failed to bootstrap service %s: %w", cfg.Name, err)
	}

	l.logger.Info("launchd service installed successfully", "service", cfg.Name)
	return nil
}

func (l *LaunchdManager) Uninstall(ctx context.Context, serviceName string) error {
	l.logger.Info("uninstalling launchd user agent", "service", serviceName)

	plistPath, err := l.getPlistPath(serviceName)
	if err != nil {
		return err
	}

	uid := os.Getuid()
	domain := fmt.Sprintf("gui/%d", uid)

	if err := l.runLaunchctl(ctx, "bootout", domain, plistPath); err != nil {
		l.logger.Warn("failed to bootout service", "service", serviceName, "error", err)
	}

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	l.logger.Info("launchd service uninstalled successfully", "service", serviceName)
	return nil
}

func (l *LaunchdManager) createPlistFile(cfg ServiceConfig, destPath string) error {
	tmpl, err := template.New("launchd").Parse(launchdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse launchd template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	logDir := filepath.Join(l.ursoHome, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, cfg)
}

func (l *LaunchdManager) getPlistPath(serviceName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(homeDir, "Library", "LaunchAgents", serviceName+".plist"), nil
}

func (l *LaunchdManager) runLaunchctl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "launchctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl command failed: %w, output: %s", err, string(output))
	}
	return nil
}

// SystemdManager implements the ServiceManager interface for Linux systems using systemd.
type SystemdManager struct {
	logger   *slog.Logger
	ursoHome string
}

func newSystemdManager(logger *slog.Logger, ursoHome string) *SystemdManager {
	return &SystemdManager{logger: logger, ursoHome: ursoHome}
}

func (s *SystemdManager) Install(ctx context.Context, cfg ServiceConfig) error {
	s.logger.Info("installing systemd user service", "service", cfg.Name)

	servicePath, err := s.getServicePath(cfg.Name)
	if err != nil {
		return err
	}

	if err := s.createServiceFile(cfg, servicePath); err != nil {
		return err
	}

	if err := s.runSystemctl(ctx, "daemon-reload"); err != nil {
		return err
	}

	if err := s.runSystemctl(ctx, "enable", "--now", cfg.Name); err != nil {
		return err
	}

	s.logger.Info("systemd service installed successfully", "service", cfg.Name)
	return nil
}

func (s *SystemdManager) Uninstall(ctx context.Context, serviceName string) error {
	s.logger.Info("uninstalling systemd user service", "service", serviceName)

	servicePath, err := s.getServicePath(serviceName)
	if err != nil {
		return err
	}

	if err := s.runSystemctl(ctx, "disable", "--now", serviceName); err != nil {
		s.logger.Warn("failed to disable service", "service", serviceName, "error", err)
	}

	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	if err := s.runSystemctl(ctx, "daemon-reload"); err != nil {
		s.logger.Warn("failed to reload systemd daemon", "error", err)
	}

	s.logger.Info("systemd service uninstalled successfully", "service", serviceName)
	return nil
}

func (s *SystemdManager) createServiceFile(cfg ServiceConfig, destPath string) error {
	tmpl, err := template.New("systemd").Parse(systemdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse systemd template: %w", err)
	}

	type systemdData struct {
		ServiceConfig
		ArgumentsJoined string
	}

	data := systemdData{
		ServiceConfig:   cfg,
		ArgumentsJoined: strings.Join(cfg.Arguments, " "),
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create systemd directory: %w", err)
	}

	logDir := filepath.Join(s.ursoHome, "logs")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create service file: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

func (s *SystemdManager) getServicePath(serviceName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "systemd", "user", serviceName+".service"), nil
}

func (s *SystemdManager) runSystemctl(ctx context.Context, args ...string) error {
	fullArgs := append([]string{"--user"}, args...)
	cmd := exec.CommandContext(ctx, "systemctl", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl command failed: %w, output: %s", err, string(output))
	}
	return nil
}
