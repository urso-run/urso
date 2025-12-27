package urso

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"
)

// --- Interfaces for Testability ---

// Syncer defines the interface for the core synchronization logic.
type Syncer interface {
	Sync(cfg Config, registerToken, removeToken string) error
}

// RunnerExecutor defines the interface for executing commands related to a runner.
// This allows us to spy on shell commands in our tests instead of actually running them.
type RunnerExecutor interface {
	Extract(archivePath, destDir string) error
	Configure(dir string, cfg RunnerConfig, token string) error
	InstallService(dir string) error
	StartService(dir string) error
	UninstallService(dir string) error
	Unconfigure(dir string, token string) error
}

// ActionsDownloader defines the interface for downloading the runner binary.
type ActionsDownloader interface {
	GetRunnerArchive(dstDir string) (string, error)
}

// MachineInspector defines the interface for inspecting the state of the local machine.
type MachineInspector interface {
	GetCurrentState(rootDir string) (MachineState, error)
	EnsureRootDirExists(rootDir string) error
	CreateTempDir(dir, pattern string) (string, error)
	RemoveAll(path string) error
	MkdirAll(path string) error
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

func (l *LiveRunnerExecutor) Extract(archivePath, destDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tar", "-xzf", archivePath, "-C", destDir)
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) Configure(dir string, cfg RunnerConfig, token string) error {
	args := []string{"--url", cfg.URL, "--token", token, "--name", cfg.Name, "--unattended", "--replace"}
	if cfg.Group != "" {
		args = append(args, "--runnergroup", cfg.Group)
	}
	if len(cfg.Labels) > 0 {
		args = append(args, "--labels", strings.Join(cfg.Labels, ","))
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./config.sh", args...)
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) InstallService(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./svc.sh", "install")
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) StartService(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./svc.sh", "start")
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) UninstallService(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./svc.sh", "uninstall")
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) Unconfigure(dir string, token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./config.sh", "remove", "--token", token)
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

// GithubAPIDownloader is the production implementation of ActionsDownloader
// that downloads the runner from the GitHub API.
type GithubAPIDownloader struct{}

func (g *GithubAPIDownloader) GetRunnerArchive(dstDir string) (string, error) {
	// This reuses the existing logic, which is now isolated.
	return getRunnerArchive(dstDir)
}

// FileSystemMachine is the production implementation of MachineInspector
// that inspects the local filesystem.
type FileSystemMachine struct{}

func (f *FileSystemMachine) GetCurrentState(rootDir string) (MachineState, error) {
	// This reuses the existing logic.
	s := MachineState{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Runners: make(map[string]struct{}),
	}
	if !supported(s.OS, s.Arch) {
		return s, fmt.Errorf("unsupported os/arch: %s/%s", s.OS, s.Arch)
	}
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		s.RootExists = false
		return s, nil
	}
	s.RootExists = true

	dirs, err := os.ReadDir(rootDir)
	if err != nil {
		return s, fmt.Errorf("error reading root dir: %v", rootDir)
	}
	for _, d := range dirs {
		s.Runners[d.Name()] = struct{}{}
	}
	return s, nil
}

// EnsureRootDirExists creates the root directory if it doesn't exist.
func (f *FileSystemMachine) EnsureRootDirExists(rootDir string) error {
	return os.MkdirAll(rootDir, 0755)
}

// CreateTempDir creates a new temporary directory in the specified directory.
func (f *FileSystemMachine) CreateTempDir(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

// RemoveAll removes a path and any children it contains.
func (f *FileSystemMachine) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// MkdirAll creates a directory path, along with any necessary parents.
func (f *FileSystemMachine) MkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

// --- Core Logic ---

// RunnerSyncer holds the dependencies for the core runner synchronization logic.
type RunnerSyncer struct {
	machine    MachineInspector
	downloader ActionsDownloader
	executor   RunnerExecutor
	logger     *slog.Logger
}

// NewRunnerSyncer creates a new RunnerSyncer with the given dependencies.
func NewRunnerSyncer(m MachineInspector, d ActionsDownloader, e RunnerExecutor, l *slog.Logger) *RunnerSyncer {
	return &RunnerSyncer{machine: m, downloader: d, executor: e, logger: l}
}

// Sync contains the core logic for adding and removing runners.
func (s *RunnerSyncer) Sync(cfg Config, registerToken, removeToken string) error {
	ms, err := s.machine.GetCurrentState(cfg.RootDir)
	if err != nil {
		return fmt.Errorf("could not get machine state: %w", err)
	}

	if !ms.RootExists {
		if err := s.machine.EnsureRootDirExists(cfg.RootDir); err != nil {
			return fmt.Errorf("error creating root dir: %w", err)
		}
	}

	runnersToCreate, runnersToRemove := s.plan(cfg, ms)

	if err := s.removeRunners(cfg.RootDir, runnersToRemove, removeToken); err != nil {
		return err // This will only be the token error
	}

	if err := s.createRunners(cfg, runnersToCreate, registerToken); err != nil {
		return err
	}

	return nil
}

// plan determines which runners need to be created and which need to be removed.
func (s *RunnerSyncer) plan(cfg Config, ms MachineState) (toCreate []RunnerConfig, toRemove map[string]struct{}) {
	toCreate = []RunnerConfig{}
	toRemove = ms.Runners
	for _, r := range cfg.Runners {
		if _, ok := toRemove[r.Name]; !ok {
			toCreate = append(toCreate, r)
		}
		delete(toRemove, r.Name)
	}
	return toCreate, toRemove
}

func (s *RunnerSyncer) removeRunners(rootDir string, runnersToRemove map[string]struct{}, removeToken string) error {
	if len(runnersToRemove) == 0 {
		return nil
	}
	s.logger.Info("runners to remove", "runners", runnersToRemove)
	if removeToken == "" {
		return errors.New("error removing runners: github-remove-token not found")
	}
	for name := range runnersToRemove {
		s.logger.Info("removing runner", "runner", name)
		if err := s.removeRunner(rootDir, name, removeToken); err != nil {
			s.logger.Warn("failed to remove runner", "runner", name, "error", err)
		}
	}
	return nil
}

func (s *RunnerSyncer) createRunners(cfg Config, runnersToCreate []RunnerConfig, registerToken string) error {
	if len(runnersToCreate) == 0 {
		return nil
	}
	s.logger.Info("runners to create", "runners", runnersToCreate)
	if registerToken == "" {
		return errors.New("error creating runners: github-register-token not found")
	}

	tempDir, err := s.machine.CreateTempDir(cfg.RootDir, "runner-archive")
	if err != nil {
		return fmt.Errorf("error creating archive dir: %w", err)
	}
	defer func() {
		if err := s.machine.RemoveAll(tempDir); err != nil {
			s.logger.Warn("failed to clean up temp dir", "path", tempDir, "error", err)
		}
	}()

	archivePath, err := s.downloader.GetRunnerArchive(tempDir)
	if err != nil {
		return fmt.Errorf("error getting runner archive: %w", err)
	}

	for _, runner := range runnersToCreate {
		if err := s.createRunner(cfg.RootDir, runner, archivePath, registerToken); err != nil {
			// Stop on the first error for creation
			return fmt.Errorf("failed to create runner %s: %w", runner.Name, err)
		}
	}
	return nil
}

func (s *RunnerSyncer) createRunner(rootDir string, cfg RunnerConfig, archive string, token string) error {
	runnerDir := path.Join(rootDir, cfg.Name)
	if err := s.machine.MkdirAll(runnerDir); err != nil {
		return fmt.Errorf("mkdir runner: %w", err)
	}
	if err := s.executor.Extract(archive, runnerDir); err != nil {
		return fmt.Errorf("extract runner: %w", err)
	}
	if err := s.executor.Configure(runnerDir, cfg, token); err != nil {
		return fmt.Errorf("configure runner: %w", err)
	}
	if err := s.executor.InstallService(runnerDir); err != nil {
		return fmt.Errorf("install runner: %w", err)
	}
	if err := s.executor.StartService(runnerDir); err != nil {
		return fmt.Errorf("start runner: %w", err)
	}
	return nil
}

func (s *RunnerSyncer) removeRunner(rootDir string, name string, token string) error {
	runnerDir := path.Join(rootDir, name)

	// Try to uninstall and unconfigure, but don't fail hard if it fails
	if err := s.executor.UninstallService(runnerDir); err != nil {
		s.logger.Warn("failed to uninstall runner service", "runner", name, "error", err)
	}
	if err := s.executor.Unconfigure(runnerDir, token); err != nil {
		s.logger.Warn("failed to unconfigure runner", "runner", name, "error", err)
	}
	if err := s.machine.RemoveAll(runnerDir); err != nil {
		return fmt.Errorf("remove runner dir: %w", err)
	}
	return nil
}
