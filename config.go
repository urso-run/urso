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
	UrsoHome() string
}

// FileSystemConfigStore is the production implementation of ConfigStore that
// interacts with the local filesystem.
type FileSystemConfigStore struct {
	ursoHome   string
	configPath string
}

// NewFileSystemConfigStore creates a new config store that operates on the
// provided ursoHome directory.
func NewFileSystemConfigStore(ursoHome string) *FileSystemConfigStore {
	configPath := filepath.Join(ursoHome, "config.yaml")

	return &FileSystemConfigStore{
		ursoHome:   ursoHome,
		configPath: configPath,
	}
}

// Exists checks if the configuration file already exists on disk.
func (f *FileSystemConfigStore) Exists() bool {
	_, err := os.Stat(f.configPath)
	return err == nil
}

// Write writes the given content to the configuration file. It will create the
// necessary directories if they don't exist.
func (f *FileSystemConfigStore) Write(content []byte) error {
	dir := filepath.Dir(f.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}
	return os.WriteFile(f.configPath, content, 0600)
}

// Read reads the configuration file from disk.
func (f *FileSystemConfigStore) Read() ([]byte, error) {
	return os.ReadFile(f.configPath)
}

// Path returns the full path to the configuration file.
func (f *FileSystemConfigStore) Path() string {
	return f.configPath
}

// UrsoHome returns the root directory for Urso.
func (f *FileSystemConfigStore) UrsoHome() string {
	return f.ursoHome
}
