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
	"text/template"
)

const (
	// ServiceName is the default name for the system service.
	ServiceName = "com.urso-run.urso"
)

// ServiceManager defines the interface for managing the `urso` system service.
type ServiceManager interface {
	// Install configures and enables the system service.
	// `executablePath` should be the full path to the `urso` binary.
	Install(ctx context.Context, executablePath string) error

	// Uninstall stops and removes the system service.
	Uninstall(ctx context.Context) error
}

// ErrUnsupportedOS is returned when an operation is attempted on an unsupported operating system.
var ErrUnsupportedOS = errors.New("unsupported operating system: only macOS and Linux are supported")

// NewServiceManager creates a new ServiceManager appropriate for the current OS.
func NewServiceManager(logger *slog.Logger) (ServiceManager, error) {
	switch runtime.GOOS {
	case "darwin":
		return newLaunchdManager(logger), nil
	case "linux":
		return newSystemdManager(logger), nil
	default:
		return nil, ErrUnsupportedOS
	}
}

const launchdTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.ServiceName}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.ExecutablePath}}</string>
        <string>run</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{.HomeDir}}/Library/Logs/{{.ServiceName}}.log</string>
    <key>StandardErrorPath</key>
    <string>{{.HomeDir}}/Library/Logs/{{.ServiceName}}.log</string>
</dict>
</plist>
`

const systemdTemplate = `[Unit]
Description=Urso Runner Manager
After=network.target

[Service]
ExecStart={{.ExecutablePath}} run
Restart=always
StandardOutput=append:{{.HomeDir}}/.urso/logs/urso.log
StandardError=append:{{.HomeDir}}/.urso/logs/urso.log

[Install]
WantedBy=default.target
`

// LaunchdManager implements the ServiceManager interface for macOS systems using launchd.
type LaunchdManager struct {
	logger *slog.Logger
}

// newLaunchdManager creates a new LaunchdManager.
func newLaunchdManager(logger *slog.Logger) *LaunchdManager {
	return &LaunchdManager{logger: logger}
}

// Install creates a launchd plist file, loads it, and starts the service.
func (l *LaunchdManager) Install(ctx context.Context, executablePath string) error {
	l.logger.Info("installing launchd user agent")

	plistPath, err := l.getPlistPath()
	if err != nil {
		return err
	}

	l.logger.Info("creating launchd plist file", "path", plistPath)
	if err := l.createPlistFile(executablePath, plistPath); err != nil {
		return err
	}

	l.logger.Info("bootstrapping and starting service", "service", ServiceName)
	// Bootout first in case it's already bootstrapped, to ensure we're using the new definition.
	uid := os.Getuid()
	domain := fmt.Sprintf("gui/%d", uid)

	if err := l.runLaunchctl(ctx, "bootout", domain, plistPath); err != nil {
		l.logger.Warn("failed to bootout existing service (this might be expected if it's the first install)", "error", err)
	}
	if err := l.runLaunchctl(ctx, "bootstrap", domain, plistPath); err != nil {
		return fmt.Errorf("failed to bootstrap service: %w", err)
	}

	l.logger.Info("launchd service installed successfully")
	return nil
}

// Uninstall stops, unloads, and removes the launchd plist file.
func (l *LaunchdManager) Uninstall(ctx context.Context) error {
	l.logger.Info("uninstalling launchd user agent")

	plistPath, err := l.getPlistPath()
	if err != nil {
		return err
	}

	l.logger.Info("stopping and booting out service", "service", ServiceName)
	uid := os.Getuid()
	domain := fmt.Sprintf("gui/%d", uid)

	if err := l.runLaunchctl(ctx, "bootout", domain, plistPath); err != nil {
		l.logger.Warn("failed to bootout service (this might be expected if it was not loaded)", "error", err)
	}

	l.logger.Info("removing launchd plist file", "path", plistPath)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	l.logger.Info("launchd service uninstalled successfully")
	return nil
}

// createPlistFile generates the launchd plist file from the template.
func (l *LaunchdManager) createPlistFile(executablePath, destPath string) error {
	tmpl, err := template.New("launchd").Parse(launchdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse launchd template: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	data := struct {
		ServiceName    string
		ExecutablePath string
		HomeDir        string
	}{
		ServiceName:    ServiceName,
		ExecutablePath: executablePath,
		HomeDir:        homeDir,
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create plist file: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

// getPlistPath returns the path where the user-specific launchd plist file should be stored.
func (l *LaunchdManager) getPlistPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(homeDir, "Library", "LaunchAgents", ServiceName+".plist"), nil
}

// runLaunchctl executes a launchctl command.
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
	logger *slog.Logger
}

func newSystemdManager(logger *slog.Logger) *SystemdManager {
	return &SystemdManager{logger: logger}
}

// Install creates a systemd service file, reloads the daemon, and starts the service.
func (s *SystemdManager) Install(ctx context.Context, executablePath string) error {
	s.logger.Info("installing systemd user service")

	servicePath, err := s.getServicePath()
	if err != nil {
		return err
	}

	if err := s.createServiceFile(executablePath, servicePath); err != nil {
		return err
	}

	s.logger.Info("reloading systemd manager")
	if err := s.runSystemctl(ctx, "daemon-reload"); err != nil {
		return err
	}

	s.logger.Info("enabling and starting service", "service", ServiceName)
	if err := s.runSystemctl(ctx, "enable", "--now", ServiceName); err != nil {
		return err
	}

	s.logger.Info("systemd service installed successfully")
	return nil
}

// Uninstall stops, disables, and removes the systemd service file.
func (s *SystemdManager) Uninstall(ctx context.Context) error {
	s.logger.Info("uninstalling systemd user service")

	servicePath, err := s.getServicePath()
	if err != nil {
		return err
	}

	s.logger.Info("stopping and disabling service", "service", ServiceName)
	if err := s.runSystemctl(ctx, "disable", "--now", ServiceName); err != nil {
		s.logger.Warn("failed to disable service", "error", err)
	}

	s.logger.Info("removing systemd service file", "path", servicePath)
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	if err := s.runSystemctl(ctx, "daemon-reload"); err != nil {
		s.logger.Warn("failed to reload systemd daemon", "error", err)
	}

	s.logger.Info("systemd service uninstalled successfully")
	return nil
}

func (s *SystemdManager) createServiceFile(executablePath, destPath string) error {
	tmpl, err := template.New("systemd").Parse(systemdTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse systemd template: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	data := struct {
		ExecutablePath string
		HomeDir        string
	}{
		ExecutablePath: executablePath,
		HomeDir:        homeDir,
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create systemd directory: %w", err)
	}

	// Also ensure log directory exists
	logDir := filepath.Join(homeDir, ".urso", "logs")
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

func (s *SystemdManager) getServicePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "systemd", "user", ServiceName+".service"), nil
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
