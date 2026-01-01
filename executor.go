package urso

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// --- Interfaces for Testability ---

// RunnerExecutor defines the interface for executing commands related to a runner.
// This allows us to spy on shell commands in our tests instead of actually running them.
type RunnerExecutor interface {
	Extract(ctx context.Context, archivePath, destDir string) error
	Configure(ctx context.Context, dir string, cfg RunnerConfig, token string) error
	InstallService(ctx context.Context, dir string) error
	StartService(ctx context.Context, dir string) error
	UninstallService(ctx context.Context, dir string) error
	Unconfigure(ctx context.Context, dir string, token string) error
}

// --- Live Implementations ---

// LiveRunnerExecutor is the production implementation of RunnerExecutor that
// actually executes shell commands.
type LiveRunnerExecutor struct {
	out io.Writer
}

// NewLiveRunnerExecutor creates a new LiveRunnerExecutor that writes command
// output to the given writer.
func NewLiveRunnerExecutor(out io.Writer) *LiveRunnerExecutor {
	return &LiveRunnerExecutor{out: out}
}

func (l *LiveRunnerExecutor) Extract(ctx context.Context, archivePath, destDir string) error {
	cmd := exec.CommandContext(ctx, "tar", "-xzf", archivePath, "-C", destDir)
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) Configure(ctx context.Context, dir string, cfg RunnerConfig, token string) error {
	args := []string{"--url", cfg.URL, "--token", token, "--name", cfg.Name, "--unattended", "--replace"}
	if cfg.Group != "" {
		args = append(args, "--runnergroup", cfg.Group)
	}
	if len(cfg.Labels) > 0 {
		args = append(args, "--labels", strings.Join(cfg.Labels, ","))
	}

	cmd := exec.CommandContext(ctx, "./config.sh", args...)
	l.prepareConfigCmd(cmd)
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) InstallService(ctx context.Context, dir string) error {
	return l.runSvc(ctx, dir, "install")
}

func (l *LiveRunnerExecutor) StartService(ctx context.Context, dir string) error {
	return l.runSvc(ctx, dir, "start")
}

func (l *LiveRunnerExecutor) UninstallService(ctx context.Context, dir string) error {
	return l.runSvc(ctx, dir, "uninstall")
}

func (l *LiveRunnerExecutor) Unconfigure(ctx context.Context, dir string, token string) error {
	cmd := exec.CommandContext(ctx, "./config.sh", "remove", "--token", token)
	l.prepareConfigCmd(cmd)
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) prepareConfigCmd(cmd *exec.Cmd) {
	// GitHub Actions config.sh fails if run as root unless RUNNER_ALLOW_RUNASROOT is set.
	if os.Geteuid() == 0 {
		cmd.Env = append(os.Environ(), "RUNNER_ALLOW_RUNASROOT=1")
	}
}

func (l *LiveRunnerExecutor) runSvc(ctx context.Context, dir string, action string) error {
	var cmd *exec.Cmd
	// svc.sh MUST run as root on Linux.
	if runtime.GOOS == "linux" && os.Geteuid() != 0 {
		cmd = exec.CommandContext(ctx, "sudo", "./svc.sh", action)
	} else {
		cmd = exec.CommandContext(ctx, "./svc.sh", action)
	}

	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("svc.sh %s failed: %w", action, err)
	}
	return nil
}
