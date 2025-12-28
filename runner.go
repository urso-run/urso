package urso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// --- Interfaces for Testability ---

// Syncer defines the interface for the core synchronization logic.
type Syncer interface {
	Sync(ctx context.Context, cfg Config, registerToken, removeToken string) error
}

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

// ActionsDownloader defines the interface for downloading the runner binary.
type ActionsDownloader interface {
	GetRunnerArchive(ctx context.Context, dstDir string) (string, error)
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
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) InstallService(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "./svc.sh", "install")
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) StartService(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "./svc.sh", "start")
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) UninstallService(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "./svc.sh", "uninstall")
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

func (l *LiveRunnerExecutor) Unconfigure(ctx context.Context, dir string, token string) error {
	cmd := exec.CommandContext(ctx, "./config.sh", "remove", "--token", token)
	cmd.Dir = dir
	cmd.Stdout = l.out
	cmd.Stderr = l.out
	return cmd.Run()
}

// GithubAPIDownloader is the production implementation of ActionsDownloader
// that downloads the runner from the GitHub API.
type GithubAPIDownloader struct {
	client *http.Client
}

// NewGithubAPIDownloader creates a new downloader.
func NewGithubAPIDownloader(client *http.Client) *GithubAPIDownloader {
	return &GithubAPIDownloader{client: client}
}

func (g *GithubAPIDownloader) GetRunnerArchive(ctx context.Context, _ string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get home directory: %w", err)
	}
	cacheDir := filepath.Join(home, ".urso", "cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	release, err := g.fetchLatestRelease(ctx)
	if err != nil {
		return "", err
	}

	archivePath := filepath.Join(cacheDir, archiveFilename)
	versionPath := filepath.Join(cacheDir, "version.txt")

	cachedVersion, _ := os.ReadFile(versionPath)
	if string(cachedVersion) == release.TagName {
		if _, err := os.Stat(archivePath); err == nil {
			return archivePath, nil
		}
	}

	downloadURL, err := g.getDownloadURL(release)
	if err != nil {
		return "", err
	}

	if err := g.download(ctx, downloadURL, archivePath); err != nil {
		return "", err
	}

	if err := os.WriteFile(versionPath, []byte(release.TagName), 0600); err != nil {
		return "", fmt.Errorf("failed to save cached version: %w", err)
	}

	return archivePath, nil
}

func (g *GithubAPIDownloader) fetchLatestRelease(ctx context.Context) (releaseResponse, error) {
	url := "https://api.github.com/repos/actions/runner/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return releaseResponse{}, fmt.Errorf("new request: %w", err)
	}

	r, err := g.client.Do(req)
	if err != nil {
		return releaseResponse{}, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return releaseResponse{}, fmt.Errorf("unexpected status code: %d", r.StatusCode)
	}

	var release releaseResponse
	if err := json.NewDecoder(r.Body).Decode(&release); err != nil {
		return releaseResponse{}, fmt.Errorf("failed to decode release info: %w", err)
	}
	return release, nil
}

func (g *GithubAPIDownloader) getDownloadURL(release releaseResponse) (string, error) {
	osPart := runtime.GOOS
	if osPart == "darwin" {
		osPart = "osx"
	}
	archPart := runtime.GOARCH
	if archPart == "amd64" {
		archPart = "x64"
	}

	search := fmt.Sprintf("actions-runner-%s-%s-", osPart, archPart)

	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, search) && (strings.HasSuffix(asset.Name, ".tar.gz")) {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("no runner found for %s/%s", runtime.GOOS, runtime.GOARCH)
}

func (g *GithubAPIDownloader) download(ctx context.Context, url, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer out.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", "go-http-client")
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	if _, err = io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
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

	tempDir, err := s.machine.CreateTempDir(cfg.RootDir, "runner-archive")
	if err != nil {
		return fmt.Errorf("error creating archive dir: %w", err)
	}
	defer func() {
		if err := s.machine.RemoveAll(tempDir); err != nil {
			s.logger.Warn("failed to clean up temp dir", "path", tempDir, "error", err)
		}
	}()

	archivePath, err := s.downloader.GetRunnerArchive(ctx, tempDir)
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
