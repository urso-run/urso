package urso

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// APIClient defines the interface for communicating with the Urso Dashboard API.
type APIClient interface {
	RegisterMachine(ctx context.Context, jwt, hostname string) (machineID string, machineToken string, err error)
	GetRunnerConfig(ctx context.Context, id, token string) ([]RunnerConfig, error)
	GetRegisterToken(ctx context.Context, id, token string) (string, error)
	GetRemoveToken(ctx context.Context, id, token string) (string, error)
}

// CredentialStore defines the interface for securely storing and retrieving
// the persistent machine credentials.
type CredentialStore interface {
	Save(id, token string) error
	Load() (id, token string, err error)
}

// --- Real Implementations ---

const httpTimeout = 30 * time.Second

// NewHTTPClient creates a new http.Client with a reasonable timeout.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: httpTimeout,
	}
}

// DashboardAPIClient is the live implementation of the APIClient that makes
// real HTTP requests to the Urso Dashboard.
type DashboardAPIClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Logger     *slog.Logger
}

// FileSystemCredentialStore is the live implementation of CredentialStore that
// saves credentials to a JSON file on the local filesystem.
type FileSystemCredentialStore struct {
	path string
}

// NewFileSystemCredentialStore creates a credential store that operates on the
// default path (~/.urso/credentials.json).
func NewFileSystemCredentialStore() (*FileSystemCredentialStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get user home directory: %w", err)
	}
	credPath := filepath.Join(home, ".urso", "credentials.json")
	return &FileSystemCredentialStore{path: credPath}, nil
}

// --- API Response Structs ---

type registerMachineRequest struct {
	Hostname string `json:"hostname"`
}

type registerMachineResponse struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

// apiRunnerConfig is the representation of a runner from the API.
type apiRunnerConfig struct {
	Name   string   `json:"name"`
	Group  string   `json:"group"`
	URL    string   `json:"url"`
	Labels []string `json:"labels"`
}

// apiConfigResponse is the response from the GET /api/machine/:id endpoint.
type apiConfigResponse struct {
	// The rootdir should absolutely not be considered from response. No upside, only downsides (configuration/security)
	Runners []apiRunnerConfig `json:"runners"`
}

// --- Method Implementations ---

// RegisterMachine sends a request to the Urso API to register a new machine.
func (c *DashboardAPIClient) RegisterMachine(ctx context.Context, jwt, hostname string) (string, string, error) {
	url := c.BaseURL + "/api/machine"

	reqData := registerMachineRequest{Hostname: hostname}
	body, err := json.Marshal(reqData)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal registration request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("failed to create machine registration request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	c.Logger.Info("registering machine with api", "url", url)
	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("failed to perform machine registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status code from machine registration: %d", resp.StatusCode)
	}

	var registerResp registerMachineResponse
	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		return "", "", fmt.Errorf("failed to decode machine registration response: %w", err)
	}

	if registerResp.ID == "" || registerResp.Token == "" {
		return "", "", errors.New("invalid response from API: missing id or token")
	}

	return registerResp.ID, registerResp.Token, nil
}

// GetRunnerConfig fetches the runner configuration from the Urso API.
func (c *DashboardAPIClient) GetRunnerConfig(ctx context.Context, id, token string) ([]RunnerConfig, error) {
	url := fmt.Sprintf("%s/api/machine/%s", c.BaseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get config request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform get config request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from get config: %d", resp.StatusCode)
	}

	var apiResp apiConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode get config response: %w", err)
	}

	// Map from the API representation to our internal RunnerConfig struct.
	runners := make([]RunnerConfig, len(apiResp.Runners))
	for i, r := range apiResp.Runners {
		runners[i] = RunnerConfig(r)
	}

	return runners, nil
}

// GetRegisterToken fetches a GitHub registration token from the Urso API.
func (c *DashboardAPIClient) GetRegisterToken(ctx context.Context, id, token string) (string, error) {
	return c.getToken(ctx, id, token, "registration-token")
}

// GetRemoveToken fetches a GitHub removal token from the Urso API.
func (c *DashboardAPIClient) GetRemoveToken(ctx context.Context, id, token string) (string, error) {
	return c.getToken(ctx, id, token, "remove-token")
}

func (c *DashboardAPIClient) getToken(ctx context.Context, id, token, tokenType string) (string, error) {
	url := fmt.Sprintf("%s/api/machine/%s/%s", c.BaseURL, id, tokenType)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create get %s request: %w", tokenType, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to perform get %s request: %w", tokenType, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code from get %s: %d", tokenType, resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode get %s response: %w", tokenType, err)
	}
	return tokenResp.Token, nil
}

func (c *DashboardAPIClient) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	const maxRetries = 3

	for i := range maxRetries {
		// If this is a retry, reset the body if it exists
		if i > 0 && req.GetBody != nil {
			newBody, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to reset request body: %w", err)
			}
			req.Body = newBody
		}

		resp, err = c.HTTPClient.Do(req)

		// Success or client-side error (4xx) - don't retry
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// If this was the last attempt, return whatever we got
		if i == maxRetries-1 {
			return resp, err
		}

		// If we got a response (but it was a 5xx), close it before retrying
		if resp != nil {
			resp.Body.Close()
		}

		backoff := time.Second * time.Duration(1<<i)
		c.Logger.Warn("api request failed, retrying",
			"attempt", i+1,
			"url", req.URL.String(),
			"error", err,
			"backoff", backoff,
		)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return resp, err
}

type credentials struct {
	MachineID    string `json:"machine_id"`
	MachineToken string `json:"machine_token"`
}

// Save securely stores the machine ID and token to the local filesystem.
func (s *FileSystemCredentialStore) Save(id, token string) error {
	creds := credentials{
		MachineID:    id,
		MachineToken: token,
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Ensure the .urso directory exists.
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create credentials directory: %w", err)
	}

	// Write the file with restricted permissions.
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}
	return nil
}

// Load retrieves the machine ID and token from the local filesystem.
func (s *FileSystemCredentialStore) Load() (string, string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("credentials file not found at %s: please run 'urso install' first", s.path)
		}
		return "", "", fmt.Errorf("failed to read credentials file: %w", err)
	}

	var creds credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", "", fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	if creds.MachineID == "" || creds.MachineToken == "" {
		return "", "", errors.New("invalid credentials file: missing machine_id or machine_token")
	}

	return creds.MachineID, creds.MachineToken, nil
}
