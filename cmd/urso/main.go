package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/repeat-dev/urso/internal"
	"gopkg.in/yaml.v3"
)

type Config struct {
	RootDir string                  `yaml:"rootDir"`
	Runners []internal.RunnerConfig `yaml:"runners"`
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

func SyncRunners(cfg Config, ms internal.MachineState, registerToken, removeToken string) error {
	if !ms.RootExists {
		if err := os.MkdirAll(cfg.RootDir, 0755); err != nil {
			return fmt.Errorf("error creating root dir: %v", cfg.RootDir)
		}
	}

	create := []internal.RunnerConfig{}
	remove := ms.Runners
	for _, r := range cfg.Runners {
		if _, ok := ms.Runners[r.Name]; !ok {
			create = append(create, r)
		}
		delete(remove, r.Name)
	}

	log.Printf("runners to remove: %+v", remove)
	if len(remove) > 0 && removeToken == "" {
		return errors.New("error removing runners: github-remove-token not found")
	}
	for name := range remove {
		log.Printf("removing runner: %s", name)
		if err := internal.RemoveRunner(cfg.RootDir, name, removeToken); err != nil {
			log.Printf("failed to remove runner %s: %v", name, err)
		}
	}

	log.Printf("runners to create: %+v", create)
	if len(create) == 0 {
		return nil
	}
	if registerToken == "" {
		return errors.New("error creating runners: github-register-token not found")
	}
	d, err := os.MkdirTemp(cfg.RootDir, "runner-archive")
	if err != nil {
		return fmt.Errorf("error creating archive dir: %w", err)
	}
	defer os.RemoveAll(d)

	archive, err := internal.GetRunnerArchive(d)
	if err != nil {
		return fmt.Errorf("error getting runner archive: %w", err)
	}

	for _, runner := range create {
		if err := internal.CreateRunner(cfg.RootDir, runner, archive, registerToken); err != nil {
			return fmt.Errorf("CreateRunner: %w", err)
		}
	}

	return nil
}

func main() {
	totalArgs := 2
	if len(os.Args) < totalArgs {
		log.Fatal("expected 'init', 'run', or 'install' subcommands")
	}

	switch os.Args[1] {
	case "init":
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		if err := initCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("parse failed: %v", err)
		}
		if err := runInit(); err != nil {
			log.Fatalf("init failed: %v", err)
		}
		log.Println("config.yaml created successfully.")
	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		configPath := runCmd.String("config", "config.yaml", "path to the configuration file")
		registerToken := runCmd.String("github-register-token", "", "token to register github actions runner at the organization level")
		removeToken := runCmd.String("github-remove-token", "", "token to remove github actions runner")
		if err := runCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("parse failed: %v", err)
		}

		if err := runRun(*configPath, *registerToken, *removeToken); err != nil {
			log.Fatalf("run failed: %v", err)
		}
	case "install":
		installCmd := flag.NewFlagSet("install", flag.ExitOnError)
		registrationToken := installCmd.String("urso-registration-token", "", "urso registration token")
		if err := installCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("parse failed: %v", err)
		}

		if err := runInstall(*registrationToken); err != nil {
			log.Fatalf("install failed: %v", err)
		}
	default:
		log.Fatal("expected 'init', 'run', or 'install' subcommands")
	}
}

func runInit() error {
	defaultConfig := `rootDir: ".urso"
runners:
  - name: "default-runner"
    labels:
      - self-hosted
      - linux
      - x64
    # url: "https://github.com/my-org"
`
	return os.WriteFile("config.yaml", []byte(defaultConfig), 0600)
}

func runRun(configPath, registerToken, removeToken string) error {
	cfg, err := NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	ms, err := internal.NewMachineState(cfg.RootDir)
	if err != nil {
		return err
	}
	log.Printf("machine state: %+v", ms)

	rt := registerToken
	if rt == "" {
		rt = os.Getenv("GITHUB_REGISTER_TOKEN")
	}
	rmt := removeToken
	if rmt == "" {
		rmt = os.Getenv("GITHUB_REMOVE_TOKEN")
	}

	if err := SyncRunners(cfg, ms, rt, rmt); err != nil {
		return fmt.Errorf("error synchronizing runners: %w", err)
	}
	return nil
}

func runInstall(token string) error {
	if token == "" {
		return errors.New("urso-registration-token is required for installation")
	}
	log.Println("The install command is a paid feature and is not yet implemented.")
	log.Println("Thank you for your interest!")
	return nil
}
