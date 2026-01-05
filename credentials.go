package urso

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CredentialStore defines the interface for securely storing and retrieving
// the persistent machine credentials.
type CredentialStore interface {
	Save(id, token string) error
	Load() (id, token string, err error)
	Delete() error
}

var ErrMissingCredentials = errors.New("credentials not found")

// FileSystemCredentialStore is the live implementation of CredentialStore that
// saves credentials to a JSON file on the local filesystem.
type FileSystemCredentialStore struct {
	path string
}

// NewFileSystemCredentialStore creates a credential store that operates on the
// provided ursoHome directory.
func NewFileSystemCredentialStore(ursoHome string) *FileSystemCredentialStore {
	credPath := filepath.Join(ursoHome, "credentials.json")
	return &FileSystemCredentialStore{path: credPath}
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
	if err := os.MkdirAll(dir, 0700); err != nil {
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
			return "", "", fmt.Errorf("credentials file not found at %s: %w", s.path, ErrMissingCredentials)
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

// Delete removes the machine credentials from the local filesystem.
func (s *FileSystemCredentialStore) Delete() error {
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove credentials file: %w", err)
	}
	return nil
}
