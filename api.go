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
	RegisterMachine(ctx context.Context, jwt string) (machineID string, machineToken string, err error)
	GetRunnerConfig(ctx context.Context, id, token string) (Config, error)
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

type registerMachineResponse struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

// --- Method Implementations ---

// RegisterMachine sends a request to the Urso API to register a new machine.
func (c *DashboardAPIClient) RegisterMachine(ctx context.Context, jwt string) (string, string, error) {
	url := c.BaseURL + "/api/machine"

	body := bytes.NewBufferString("{}")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", "", fmt.Errorf("failed to create machine registration request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Length", "2")

	c.Logger.Info("registering machine with api", "url", url)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to perform machine registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
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

// GetRunnerConfig is not yet implemented.
func (c *DashboardAPIClient) GetRunnerConfig(_ context.Context, id, token string) (Config, error) {
	// TODO: Implement the GET /api/machine/:id call.
	_, _ = id, token
	return Config{}, errors.New("GetRunnerConfig not implemented")
}

// GetRegisterToken is not yet implemented.
func (c *DashboardAPIClient) GetRegisterToken(_ context.Context, id, token string) (string, error) {
	// TODO: Implement the GET /api/machine/:id/registration-token call.
	_, _ = id, token
	return "", errors.New("GetRegisterToken not implemented")
}

// GetRemoveToken is not yet implemented.
func (c *DashboardAPIClient) GetRemoveToken(_ context.Context, id, token string) (string, error) {
	// TODO: Implement the GET /api/machine/:id/remove-token call.
	_, _ = id, token
	return "", errors.New("GetRemoveToken not implemented")
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
