package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/repeat-dev/urso"
)

// These variables are set at build time via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// commandTimeout defines the maximum execution time for a command.
const commandTimeout = 5 * time.Minute

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Create dependencies
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store, err := urso.NewFileSystemConfigStore()
	if err != nil {
		return fmt.Errorf("failed to initialize configuration store: %w", err)
	}
	machine := &urso.FileSystemMachine{}
	httpClient := urso.NewHTTPClient()
	downloader := urso.NewGithubAPIDownloader(httpClient)
	executor := urso.NewLiveRunnerExecutor(os.Stdout)
	syncer := urso.NewRunnerSyncer(machine, downloader, executor, logger)
	sm, err := urso.NewServiceManager(logger)
	if err != nil {
		logger.Warn("could not initialize service manager", "error", err)
	}
	apiClient := &urso.DashboardAPIClient{
		BaseURL:    "https://urso.run",
		HTTPClient: httpClient,
		Logger:     logger,
	}
	credStore, err := urso.NewFileSystemCredentialStore()
	if err != nil {
		return fmt.Errorf("failed to initialize credential store: %w", err)
	}
	cli := urso.NewCLI(os.Stdin, os.Stdout, os.Stderr, store, syncer, sm, apiClient, credStore, logger, version, commit, date)

	// 2. Define flags
	fs := flag.NewFlagSet("urso", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", defaultConfigPath(), "path to the configuration file")
	registerToken := fs.String("github-register-token", "", "token to register github actions runner")
	removeToken := fs.String("github-remove-token", "", "token to remove github actions runner")
	installToken := fs.String("urso-registration-token", "", "urso registration token")

	// 3. Parse command and flags
	const minArgs = 2
	if len(os.Args) < minArgs {
		cli.PrintUsage()
		return errors.New("a command is required")
	}
	command := os.Args[1]
	if err := fs.Parse(os.Args[2:]); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	// 4. Execute command with a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	switch command {
	case "init":
		return cli.Init()
	case "run":
		regToken := urso.ResolveToken(*registerToken, urso.EnvVarRegisterToken)
		remToken := urso.ResolveToken(*removeToken, urso.EnvVarRemoveToken)
		return cli.Run(ctx, *configPath, regToken, remToken)
	case "install":
		return cli.Install(ctx, *installToken)
	case "version":
		cli.Version()
	case "help":
		cli.PrintUsage()
	default:
		cli.PrintUsage()
		return fmt.Errorf("unknown command: '%s'", command)
	}
	return nil
}

// defaultConfigPath returns the default path for the config file.
func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// This is unlikely to fail, but if it does, we'll fall back.
		return "config.yaml"
	}
	return filepath.Join(home, ".urso", "config.yaml")
}
