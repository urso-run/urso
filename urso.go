package urso

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

// private functions used by the live implementations in runner.go
// TODO: in the future, these could be unexported methods on a struct that holds dependencies like the http client.

func supported(os, arch string) bool {
	switch strings.Join([]string{os, arch}, "/") {
	case "darwin/arm64", "linux/amd64", "linux/arm64":
		return true
	default:
		return false
	}
}

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
