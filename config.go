package urso

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigStore defines the interface for persisting and retrieving configuration.
// This allows for swapping out the real filesystem for a spy in tests.
type ConfigStore interface {
	Exists() bool
	Read() ([]byte, error)
	Write(content []byte) error
	Path() string
}

// FileSystemConfigStore is the production implementation of ConfigStore that
// interacts with the local filesystem.
type FileSystemConfigStore struct {
	path string
}

// NewFileSystemConfigStore creates a new config store that operates on the
// default config file path (~/.urso/config.yaml).
func NewFileSystemConfigStore() (*FileSystemConfigStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get user home directory: %w", err)
	}
	configPath := filepath.Join(home, ".urso", "config.yaml")

	return &FileSystemConfigStore{path: configPath}, nil
}

// Exists checks if the configuration file already exists on disk.
func (f *FileSystemConfigStore) Exists() bool {
	_, err := os.Stat(f.path)
	return err == nil
}

// Write writes the given content to the configuration file. It will create the
// necessary directories if they don't exist.
func (f *FileSystemConfigStore) Write(content []byte) error {
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	return os.WriteFile(f.path, content, 0600)
}

// Read reads the configuration file from disk.
func (f *FileSystemConfigStore) Read() ([]byte, error) {
	return os.ReadFile(f.path)
}

// Path returns the full path to the configuration file.
func (f *FileSystemConfigStore) Path() string {
	return f.path
}
