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
	"time"
)

const (
	// ServiceName is the default name for the system service.
	ServiceName = "com.repeat.urso"
)

// ServiceManager defines the interface for managing the `urso` system service.
type ServiceManager interface {
	// Install configures and enables the system service.
	// `executablePath` should be the full path to the `urso` binary.
	Install(executablePath string) error

	// Uninstall stops and removes the system service.
	Uninstall() error
}

// ErrUnsupportedOS is returned when an operation is attempted on an unsupported operating system.
var ErrUnsupportedOS = errors.New("unsupported operating system: only macOS (darwin) is supported")

// NewServiceManager creates a new ServiceManager appropriate for the current OS.
// It returns ErrUnsupportedOS if the OS is not macOS.
func NewServiceManager(logger *slog.Logger) (ServiceManager, error) {
	if runtime.GOOS != "darwin" {
		return nil, ErrUnsupportedOS
	}
	manager := newLaunchdManager(logger)
	return manager, nil
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

// LaunchdManager implements the ServiceManager interface for macOS systems using launchd.
type LaunchdManager struct {
	logger *slog.Logger
}

// newLaunchdManager creates a new LaunchdManager.
func newLaunchdManager(logger *slog.Logger) *LaunchdManager {
	return &LaunchdManager{logger: logger}
}

// Install creates a launchd plist file, loads it, and starts the service.
func (l *LaunchdManager) Install(executablePath string) error {
	l.logger.Info("installing launchd user agent")

	plistPath, err := l.getPlistPath()
	if err != nil {
		return err
	}

	l.logger.Info("creating launchd plist file", "path", plistPath)
	if err := l.createPlistFile(executablePath, plistPath); err != nil {
		return err
	}

	l.logger.Info("loading and starting service", "service", ServiceName)
	// Unload first in case it's already loaded, to ensure we're using the new definition.
	if err := l.runLaunchctl("unload", plistPath); err != nil {
		l.logger.Warn("failed to unload existing service (this might be expected if it's the first install)", "error", err)
	}
	if err := l.runLaunchctl("load", plistPath); err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}
	if err := l.runLaunchctl("start", ServiceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	l.logger.Info("launchd service installed successfully")
	return nil
}

// Uninstall stops, unloads, and removes the launchd plist file.
func (l *LaunchdManager) Uninstall() error {
	l.logger.Info("uninstalling launchd user agent")

	plistPath, err := l.getPlistPath()
	if err != nil {
		return err
	}

	l.logger.Info("stopping and unloading service", "service", ServiceName)
	if err := l.runLaunchctl("stop", ServiceName); err != nil {
		l.logger.Warn("failed to stop service (this might be expected if it was not running)", "error", err)
	}
	if err := l.runLaunchctl("unload", plistPath); err != nil {
		l.logger.Warn("failed to unload service (this might be expected if it was not loaded)", "error", err)
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
func (l *LaunchdManager) runLaunchctl(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "launchctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl command failed: %w, output: %s", err, string(output))
	}
	return nil
}
