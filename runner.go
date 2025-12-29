package urso

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
)

// --- Interfaces for Testability ---

// Syncer defines the interface for the core synchronization logic.
type Syncer interface {
	Sync(ctx context.Context, cfg Config, registerToken, removeToken string) error
}

// --- Live Implementations ---

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
func (s *RunnerSyncer) Sync(ctx context.Context, cfg Config, registerToken, removeToken string) error {
	for _, r := range cfg.Runners {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("invalid runner configuration: %w", err)
		}
	}

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

	errRemove := s.removeRunners(ctx, cfg.RootDir, runnersToRemove, removeToken)
	errCreate := s.createRunners(ctx, cfg, runnersToCreate, registerToken)

	return errors.Join(errRemove, errCreate)
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

func (s *RunnerSyncer) removeRunners(ctx context.Context, rootDir string, runnersToRemove map[string]struct{}, removeToken string) error {
	if len(runnersToRemove) == 0 {
		return nil
	}
	s.logger.Info("runners to remove", "runners", runnersToRemove)
	if removeToken == "" {
		return errors.New("error removing runners: github-remove-token not found")
	}
	var errs []error
	for name := range runnersToRemove {
		s.logger.Info("removing runner", "runner", name)
		if err := s.removeRunner(ctx, rootDir, name, removeToken); err != nil {
			s.logger.Error("failed to remove runner", "runner", name, "error", err)
			errs = append(errs, fmt.Errorf("failed to remove runner %s: %w", name, err))
		}
	}
	return errors.Join(errs...)
}

func (s *RunnerSyncer) createRunners(ctx context.Context, cfg Config, runnersToCreate []RunnerConfig, registerToken string) error {
	if len(runnersToCreate) == 0 {
		return nil
	}
	s.logger.Info("runners to create", "runners", runnersToCreate)
	if registerToken == "" {
		return errors.New("error creating runners: github-register-token not found")
	}

	cacheDir := path.Join(cfg.RootDir, ".cache")
	if err := s.machine.MkdirAll(cacheDir); err != nil {
		return fmt.Errorf("error creating cache dir: %w", err)
	}

	archivePath, err := s.downloader.GetRunnerArchive(ctx, cacheDir)
	if err != nil {
		return fmt.Errorf("error getting runner archive: %w", err)
	}

	var errs []error
	for _, runner := range runnersToCreate {
		if err := s.createRunner(ctx, cfg.RootDir, runner, archivePath, registerToken); err != nil {
			s.logger.Error("failed to create runner", "runner", runner.Name, "error", err)
			errs = append(errs, fmt.Errorf("failed to create runner %s: %w", runner.Name, err))
		}
	}
	return errors.Join(errs...)
}

func (s *RunnerSyncer) createRunner(ctx context.Context, rootDir string, cfg RunnerConfig, archive string, token string) error {
	runnerDir := path.Join(rootDir, cfg.Name)
	if err := s.machine.MkdirAll(runnerDir); err != nil {
		return fmt.Errorf("mkdir runner: %w", err)
	}
	if err := os.Chmod(runnerDir, 0700); err != nil {
		if os.IsNotExist(err) {
			s.logger.Warn("runner dir missing during chmod, skipping", "dir", runnerDir, "error", err)
		} else {
			return fmt.Errorf("chmod runner dir: %w", err)
		}
	}
	if err := s.executor.Extract(ctx, archive, runnerDir); err != nil {
		return fmt.Errorf("extract runner: %w", err)
	}
	if err := s.executor.Configure(ctx, runnerDir, cfg, token); err != nil {
		return fmt.Errorf("configure runner: %w", err)
	}
	if err := s.executor.InstallService(ctx, runnerDir); err != nil {
		return fmt.Errorf("install runner: %w", err)
	}
	if err := s.executor.StartService(ctx, runnerDir); err != nil {
		return fmt.Errorf("start runner: %w", err)
	}
	return nil
}

func (s *RunnerSyncer) removeRunner(ctx context.Context, rootDir string, name string, token string) error {
	runnerDir := path.Join(rootDir, name)

	// Try to uninstall and unconfigure, but don't fail hard if it fails
	if err := s.executor.UninstallService(ctx, runnerDir); err != nil {
		s.logger.Warn("failed to uninstall runner service", "runner", name, "error", err)
	}
	if err := s.executor.Unconfigure(ctx, runnerDir, token); err != nil {
		s.logger.Warn("failed to unconfigure runner", "runner", name, "error", err)
	}
	if err := s.machine.RemoveAll(runnerDir); err != nil {
		return fmt.Errorf("remove runner dir: %w", err)
	}
	return nil
}
