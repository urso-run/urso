package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	archiveFilename = "actions-runner.tar.gz"
)

type Config struct {
	RootDir string `yaml:"rootDir"`
	Runners []RunnerConfig
}

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
		return fmt.Errorf("error removing runners: github-remove-token not found")
	}
	for name := range remove {
		log.Printf("removing runner: %s", name)
		if err := RemoveRunner(cfg.RootDir, name, removeToken); err != nil {
			log.Printf("failed to remove runner %s: %v", name, err)
		}
	}

	log.Printf("runners to create: %+v", create)
	if len(create) == 0 {
		return nil
	}
	if registerToken == "" {
		return fmt.Errorf("error creating runners: github-register-token not found")
	}
	d, err := os.MkdirTemp(cfg.RootDir, "runner-archive")
	if err != nil {
		return fmt.Errorf("error creating archive dir: %w", err)
	}
	defer os.RemoveAll(d)

	archive, err := GetRunnerArchive(d)
	if err != nil {
		return fmt.Errorf("error getting runner archive: %w", err)
	}

	for _, runner := range create {
		if err := CreateRunner(cfg.RootDir, runner, archive, registerToken); err != nil {
			return fmt.Errorf("CreateRunner: %w", err)
		}
	}

	return nil
}

func main() {
	configPath := flag.String("config", "config.yaml", "path to the configuration file")
	registerToken := flag.String("github-register-token", "", "token to register github actions runner at the organization level")
	removeToken := flag.String("github-remove-token", "", "token to remove github actions runner")
	flag.Parse()

	cfg, err := NewConfig(*configPath)
	if err != nil {
		log.Fatalf("error loading config: %s", err)
	}

	ms, err := NewMachineState(cfg.RootDir)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("machine state: %+v", ms)

	rt := *registerToken
	if rt == "" {
		rt = os.Getenv("GITHUB_REGISTER_TOKEN")
	}
	rmt := *removeToken
	if rmt == "" {
		rmt = os.Getenv("GITHUB_REMOVE_TOKEN")
	}

	if err := SyncRunners(cfg, ms, rt, rmt); err != nil {
		log.Printf("error synchronizing runners: %v", err)
	}
}
