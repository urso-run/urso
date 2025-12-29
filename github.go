package urso

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// releaseResponse is used for decoding the GitHub API response for runner releases.
type releaseResponse struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// ActionsDownloader defines the interface for downloading the runner binary.
type ActionsDownloader interface {
	GetRunnerArchive(ctx context.Context, dstDir string) (string, error)
}

// GithubAPIDownloader is the production implementation of ActionsDownloader
// that downloads the runner from the GitHub API.
type GithubAPIDownloader struct {
	client *http.Client
	logger *slog.Logger
}

// NewGithubAPIDownloader creates a new downloader.
func NewGithubAPIDownloader(client *http.Client, logger *slog.Logger) *GithubAPIDownloader {
	return &GithubAPIDownloader{client: client, logger: logger}
}

func (g *GithubAPIDownloader) GetRunnerArchive(ctx context.Context, dstDir string) (string, error) {
	cacheDir := dstDir
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not get home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".urso", "cache")
	}
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
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
			g.logger.Info("using cached runner archive", "version", release.TagName)
			return archivePath, nil
		}
	}

	g.logger.Info("downloading new runner archive", "version", release.TagName, "cache_dir", cacheDir)
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
