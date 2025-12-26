package urso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	archiveFilename       = "actions-runner.tar.gz"
	requestTimeoutSeconds = 30
	commandTimeoutSeconds = 30
)

// RunnerConfig defines the configuration for a single GitHub Actions runner.
type RunnerConfig struct {
	Name   string   `yaml:"name"`
	Group  string   `yaml:"group"`
	URL    string   `yaml:"url"`
	Labels []string `yaml:"labels"`
}

// Config defines the root configuration for the application.
type Config struct {
	RootDir string         `yaml:"rootDir"`
	Runners []RunnerConfig `yaml:"runners"`
}

// MachineState represents the current state of runners on the machine.
type MachineState struct {
	OS         string
	Arch       string
	RootExists bool
	Runners    map[string]struct{}
}

// NewConfig reads and parses the configuration file from the given path.
func NewConfig(configPath string) (Config, error) {
	f, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(f, &cfg); err != nil {
		return Config{}, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	cfg.RootDir = filepath.Join(home, cfg.RootDir)

	return cfg, nil
}

// SyncRunners is the core logic function that synchronizes the state of runners
// on the machine with the desired state from the configuration.
func SyncRunners(cfg Config, ms MachineState, registerToken, removeToken string) error {
	if !ms.RootExists {
		if err := os.MkdirAll(cfg.RootDir, 0755); err != nil {
			return fmt.Errorf("error creating root dir: %v", cfg.RootDir)
		}
	}

	create := []RunnerConfig{}
	remove := ms.Runners
	for _, r := range cfg.Runners {
		if _, ok := ms.Runners[r.Name]; !ok {
			create = append(create, r)
		}
		delete(remove, r.Name)
	}

	log.Printf("runners to remove: %+v", remove)
	if len(remove) > 0 && removeToken == "" {
		return errors.New("error removing runners: github-remove-token not found")
	}
	for name := range remove {
		log.Printf("removing runner: %s", name)
		if err := removeRunner(cfg.RootDir, name, removeToken); err != nil {
			log.Printf("failed to remove runner %s: %v", name, err)
		}
	}

	log.Printf("runners to create: %+v", create)
	if len(create) == 0 {
		return nil
	}
	if registerToken == "" {
		return errors.New("error creating runners: github-register-token not found")
	}
	d, err := os.MkdirTemp(cfg.RootDir, "runner-archive")
	if err != nil {
		return fmt.Errorf("error creating archive dir: %w", err)
	}
	defer os.RemoveAll(d)

	archive, err := getRunnerArchive(d)
	if err != nil {
		return fmt.Errorf("error getting runner archive: %w", err)
	}

	for _, runner := range create {
		if err := createRunner(cfg.RootDir, runner, archive, registerToken); err != nil {
			return fmt.Errorf("CreateRunner: %w", err)
		}
	}

	return nil
}

// NewMachineState discovers the current state of the machine, including OS,
// architecture, and existing runners.
func NewMachineState(rootDir string) (MachineState, error) {
	s := MachineState{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Runners: map[string]struct{}{},
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

func supported(os, arch string) bool {
	switch strings.Join([]string{os, arch}, "/") {
	case "darwin/arm64":
		return true
	default:
		return false
	}
}

// --- Runner Actions ---

type releaseResponse struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func getLatestRunnerURL() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeoutSeconds*time.Second)
	defer cancel()

	url := "https://api.github.com/repos/actions/runner/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}

	c := http.Client{
		Timeout: requestTimeoutSeconds * time.Second,
	}
	r, err := c.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", r.StatusCode)
	}

	var release releaseResponse
	if err := json.NewDecoder(r.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to decode release info: %w", err)
	}

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

func getRunnerArchive(dstDir string) (string, error) {
	url, err := getLatestRunnerURL()
	if err != nil {
		return "", fmt.Errorf("error getting latest runner url: %w", err)
	}

	archive := filepath.Join(dstDir, archiveFilename)
	out, err := os.Create(archive)
	if err != nil {
		return "", fmt.Errorf("error creating file: %w", err)
	}
	defer out.Close()

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeoutSeconds*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("User-Agent", "go-http-client")
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	if _, err = io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("copy: %w", err)
	}

	return archive, nil
}

func createRunner(rootDir string, cfg RunnerConfig, archive string, token string) error {
	runnerDir := path.Join(rootDir, cfg.Name)
	if err := os.MkdirAll(runnerDir, 0755); err != nil {
		return fmt.Errorf("mkdir runner: %w", err)
	}
	if err := extractRunner(archive, runnerDir); err != nil {
		return fmt.Errorf("extract runner: %w", err)
	}
	if err := configureRunner(runnerDir, cfg, token); err != nil {
		return fmt.Errorf("configure runner: %w", err)
	}
	if err := installRunnerSvc(runnerDir); err != nil {
		return fmt.Errorf("install runner: %w", err)
	}
	if err := startRunnerSvc(runnerDir); err != nil {
		return fmt.Errorf("start runner: %w", err)
	}
	return nil
}

func extractRunner(archivePath, destDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tar", "-xzf", archivePath, "-C", destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func configureRunner(dir string, cfg RunnerConfig, token string) error {
	args := []string{
		"--url", cfg.URL,
		"--token", token,
		"--name", cfg.Name,
		"--unattended",
		"--replace",
	}

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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installRunnerSvc(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./svc.sh", "install")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func startRunnerSvc(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./svc.sh", "start")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func removeRunner(rootDir string, name string, token string) error {
	runnerDir := path.Join(rootDir, name)
	if err := uninstallRunnerSvc(runnerDir); err != nil {
		log.Printf("Warning: failed to uninstall runner %s: %v\n", name, err)
	}
	if err := unconfigureRunner(runnerDir, token); err != nil {
		log.Printf("Warning: failed to unconfigure runner %s: %v\n", name, err)
	}
	if err := os.RemoveAll(runnerDir); err != nil {
		return fmt.Errorf("remove runner dir: %w", err)
	}
	return nil
}

func uninstallRunnerSvc(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./svc.sh", "uninstall")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func unconfigureRunner(dir string, token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeoutSeconds*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "./config.sh", "remove", "--token", token)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
