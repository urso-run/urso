package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type releaseResponse struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func GetLatestRunnerURL() (string, error) {
	r, err := http.Get("https://api.github.com/repos/actions/runner/releases/latest")
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

func GetRunnerArchive(dstDir string) (string, error) {
	url, err := GetLatestRunnerURL()
	if err != nil {
		return "", fmt.Errorf("error getting latest runner url: %w", err)
	}

	archive := filepath.Join(dstDir, archiveFilename)
	out, err := os.Create(archive)
	if err != nil {
		return "", fmt.Errorf("error creating file: %w", err)
	}
	defer out.Close()

	req, err := http.NewRequest("GET", url, nil)
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

type RunnerConfig struct {
	Name   string   `yaml:"name"`
	Group  string   `yaml:"group"`
	URL    string   `yaml:"url"`
	Labels []string `yaml:"labels"`
}

func CreateRunner(rootDir string, cfg RunnerConfig, archive string, token string) error {
	runnerDir := path.Join(rootDir, cfg.Name)
	if err := os.MkdirAll(runnerDir, 0755); err != nil {
		return fmt.Errorf("mkdir runner: %w", err)
	}
	if err := ExtractRunner(archive, runnerDir); err != nil {
		return fmt.Errorf("extract runner: %w", err)
	}
	if err := ConfigureRunner(runnerDir, cfg, token); err != nil {
		return fmt.Errorf("configure runner: %w", err)
	}
	if err := InstallRunnerSvc(runnerDir); err != nil {
		return fmt.Errorf("install runner: %w", err)
	}

	if err := StartRunnerSvc(runnerDir); err != nil {
		return fmt.Errorf("start runner: %w", err)
	}
	return nil
}

func ExtractRunner(archivePath, destDir string) error {
	cmd := exec.Command("tar", "-xzf", archivePath, "-C", destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ConfigureRunner(dir string, cfg RunnerConfig, token string) error {
	// ./config.sh --url <url> --token <token> --name <name> --labels <labels> --unattended --replace
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

	cmd := exec.Command("./config.sh", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func InstallRunnerSvc(dir string) error {
	// ./svc.sh install
	cmd := exec.Command("./svc.sh", "install")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func UninstallRunnerSvc(dir string) error {
	// ./svc.sh uninstall
	cmd := exec.Command("./svc.sh", "uninstall")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func StartRunnerSvc(dir string) error {
	// ./svc.sh start
	cmd := exec.Command("./svc.sh", "start")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RemoveRunner(rootDir string, name string, token string) error {
	runnerDir := path.Join(rootDir, name)

	// Try to uninstall and unconfigure, but don't fail hard if it fails (e.g. if partial install)
	if err := UninstallRunnerSvc(runnerDir); err != nil {
		fmt.Printf("Warning: failed to uninstall runner %s: %v\n", name, err)
	}

	if err := UnconfigureRunner(runnerDir, token); err != nil {
		fmt.Printf("Warning: failed to unconfigure runner %s: %v\n", name, err)
	}

	if err := os.RemoveAll(runnerDir); err != nil {
		return fmt.Errorf("remove runner dir: %w", err)
	}
	return nil
}

func UnconfigureRunner(dir string, token string) error {
	// ./config.sh remove --token <token>
	cmd := exec.Command("./config.sh", "remove", "--token", token)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
