package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/repeat-dev/urso"
)

// These variables are set at build time via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// 1. Create dependencies
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	store, err := urso.NewFileSystemConfigStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize configuration store: %v\n", err)
		os.Exit(1)
	}
	machine := &urso.FileSystemMachine{}
	downloader := &urso.GithubAPIDownloader{}
	executor := urso.NewLiveRunnerExecutor(os.Stdout)
	syncer := urso.NewRunnerSyncer(machine, downloader, executor, logger)
	sm, err := urso.NewServiceManager(logger)
	if err != nil {
		logger.Warn("could not initialize service manager", "error", err)
	}
	apiClient := &urso.DashboardAPIClient{
		HTTPClient: urso.NewHTTPClient(),
		Logger:     logger,
	}
	credStore, err := urso.NewFileSystemCredentialStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize credential store: %v\n", err)
		os.Exit(1)
	}

	cli := urso.NewCLI(os.Stdin, os.Stdout, os.Stderr, store, syncer, sm, apiClient, credStore, logger, version, commit, date)

	// 2. Define all possible flags using a single flag set.
	// Commands will simply use the flags they need.
	fs := flag.NewFlagSet("urso", flag.ExitOnError)
	fs.SetOutput(os.Stderr) // Send flag errors to stderr
	configPath := fs.String("config", defaultConfigPath(), "path to the configuration file")
	registerToken := fs.String("github-register-token", "", "token to register github actions runner")
	removeToken := fs.String("github-remove-token", "", "token to remove github actions runner")
	installToken := fs.String("urso-registration-token", "", "urso registration token")

	// 3. Parse command and flags
	const minArgs = 2
	if len(os.Args) < minArgs {
		cli.PrintUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	// Parse the flags from the arguments that come *after* the command
	if err := fs.Parse(os.Args[2:]); err != nil {
		// The flag set's ExitOnError will handle printing the error and exiting.
		// We still return an error for robustness, though it's unlikely to be reached.
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// 4. Execute the command
	switch command {
	case "init":
		err = cli.Init()
	case "run":
		regToken := urso.ResolveToken(*registerToken, urso.EnvVarRegisterToken)
		remToken := urso.ResolveToken(*removeToken, urso.EnvVarRemoveToken)
		err = cli.Run(*configPath, regToken, remToken)
	case "install":
		err = cli.Install(*installToken)
	case "version":
		cli.Version()
	case "help":
		cli.PrintUsage()
	default:
		cli.PrintUsage()
		err = fmt.Errorf("unknown command: '%s'", command)
	}

	// 5. Handle any errors from command execution
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
