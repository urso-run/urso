package urso

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

// Validate ensures the runner configuration has all required fields.
func (r RunnerConfig) Validate() error {
	if r.Name == "" {
		return errors.New("runner name is required")
	}
	if r.URL == "" {
		return fmt.Errorf("runner %q: url is required", r.Name)
	}
	return nil
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
	f, err := os.Open(configPath)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	cfg, err := ParseConfig(f)
	if err != nil {
		return Config{}, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	cfg.ExpandPaths(home)

	return cfg, nil
}

// ParseConfig decodes the YAML configuration from an io.Reader.
func ParseConfig(r io.Reader) (Config, error) {
	var cfg Config
	if err := yaml.NewDecoder(r).Decode(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// ExpandPaths resolves relative paths in the configuration.
func (c *Config) ExpandPaths(homeDir string) {
	if !filepath.IsAbs(c.RootDir) {
		c.RootDir = filepath.Join(homeDir, c.RootDir)
	}
}
