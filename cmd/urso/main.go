package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/repeat-dev/urso"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	const minArgs = 2
	if len(os.Args) < minArgs {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		initCmd.Usage = func() {
			fmt.Fprintln(os.Stderr, "Usage: urso init")
			fmt.Fprintln(os.Stderr, "Creates a default config.yaml in ~/.urso/config.yaml")
		}
		if err := initCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("parse failed: %v", err)
		}
		if err := runInit(); err != nil {
			log.Fatalf("init failed: %v", err)
		}
	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		runCmd.Usage = func() {
			fmt.Fprintln(os.Stderr, "Usage: urso run [options]")
			fmt.Fprintln(os.Stderr, "Synchronizes runners based on the config file.")
			fmt.Fprintln(os.Stderr, "\nOptions:")
			runCmd.PrintDefaults()
		}
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("could not get user home directory: %v", err)
		}
		defaultConfigPath := filepath.Join(home, ".urso", "config.yaml")
		configPath := runCmd.String("config", defaultConfigPath, "path to the configuration file")
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
		installCmd.Usage = func() {
			fmt.Fprintln(os.Stderr, "Usage: urso install [options]")
			fmt.Fprintln(os.Stderr, "Installs urso as a service (requires a paid license).")
			fmt.Fprintln(os.Stderr, "\nOptions:")
			installCmd.PrintDefaults()
		}
		registrationToken := installCmd.String("urso-registration-token", "", "urso registration token")
		if err := installCmd.Parse(os.Args[2:]); err != nil {
			log.Fatalf("parse failed: %v", err)
		}

		if err := runInstall(*registrationToken); err != nil {
			log.Fatalf("install failed: %v", err)
		}
	case "version":
		fmt.Fprintf(os.Stdout, "urso version %s, commit %s, built at %s\n", version, commit, date)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: urso <command> [arguments]

Available commands:
  init      Create a default config.yaml for runners
  run       Run the sync to create/remove runners based on config.yaml
  install   Install urso as a service (paid license only)
  version   Print the version number
  help      Show this help message
`)
}

func runInit() error {
	store, err := urso.NewFileSystemConfigStore()
	if err != nil {
		return err
	}
	cli := urso.NewCLI(os.Stdin, os.Stdout, store)
	return cli.Init()
}

func runRun(configPath, registerToken, removeToken string) error {
	cfg, err := urso.NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	machine := &urso.FileSystemMachine{}
	downloader := &urso.GithubAPIDownloader{}
	executor := &urso.LiveRunnerExecutor{}

	syncer := urso.NewRunnerSyncer(machine, downloader, executor)

	rt := registerToken
	if rt == "" {
		rt = os.Getenv("GITHUB_REGISTER_TOKEN")
	}
	rmt := removeToken
	if rmt == "" {
		rmt = os.Getenv("GITHUB_REMOVE_TOKEN")
	}

	if err := syncer.Sync(cfg, rt, rmt); err != nil {
		return fmt.Errorf("error synchronizing runners: %w", err)
	}
	return nil
}

func runInstall(token string) error {
	log.Println("The install command is a paid feature and is not yet implemented.")
	log.Println("Thank you for your interest!")
	if token == "" {
		return errors.New("urso-registration-token is required for installation")
	}
	return nil
}
