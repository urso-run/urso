package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/urso-run/urso"
)

// These variables are set at build time via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

const commandTimeout = 5 * time.Minute

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		ursoHome      string
		registerToken string
		removeToken   string
		installToken  string
	)

	rootCmd := &cobra.Command{
		Use:           "urso",
		Short:         "Urso is a GitHub Actions runner manager",
		Version:       fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		SilenceErrors: true,
	}

	// Persistent flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&ursoHome, "urso-home", defaultUrsoHome(), "base directory for urso configuration and state")

	// Standard streams
	out := os.Stdout
	errOut := os.Stderr
	in := os.Stdin

	// Dependencies
	// Info logs go to stdout; errors returned by commands are printed to stderr in main().
	// When running under launchd, both are captured into the same log file.
	logger := slog.New(slog.NewTextHandler(out, nil))

	store := urso.NewFileSystemConfigStore(ursoHome)
	httpClient := urso.NewHTTPClient()
	apiClient := &urso.DashboardAPIClient{
		BaseURL:    "https://urso.run",
		HTTPClient: httpClient,
		Logger:     logger,
	}
	credStore := urso.NewFileSystemCredentialStore(ursoHome)
	syncer := urso.NewRunnerSyncer(
		&urso.FileSystemMachine{},
		urso.NewGithubAPIDownloader(httpClient, logger),
		urso.NewLiveRunnerExecutor(out),
		logger,
	)
	sm, err := urso.NewServiceManager(logger, ursoHome)
	if err != nil {
		logger.Warn("service manager unavailable", "error", err)
	}

	cli := urso.NewCLI(in, out, errOut, store, syncer, sm, apiClient, credStore, logger)

	// Command: init
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create a default config.yaml for runners",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			return cli.Init()
		},
	}

	// Command: run
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the sync to create/remove runners based on config.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			ctx, cancel := context.WithTimeout(cmd.Context(), commandTimeout)
			defer cancel()

			reg := urso.ResolveToken(registerToken, urso.EnvVarRegisterToken)
			rem := urso.ResolveToken(removeToken, urso.EnvVarRemoveToken)

			return cli.Run(ctx, store.Path(), reg, rem)
		},
	}
	runCmd.Flags().StringVar(&registerToken, "github-register-token", "", "token to register github actions runner")
	runCmd.Flags().StringVar(&removeToken, "github-remove-token", "", "token to remove github actions runner")

	// Command: install
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install urso as a service (managed/cloud only)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			ctx, cancel := context.WithTimeout(cmd.Context(), commandTimeout)
			defer cancel()

			return cli.Install(ctx, installToken)
		},
	}
	installCmd.Flags().StringVar(&installToken, "urso-registration-token", "", "urso registration token")

	// Command: uninstall
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall urso service",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			ctx, cancel := context.WithTimeout(cmd.Context(), commandTimeout)
			defer cancel()

			return cli.Uninstall(ctx)
		},
	}

	rootCmd.AddCommand(initCmd, runCmd, installCmd, uninstallCmd)

	return rootCmd
}

func defaultUrsoHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".urso"
	}
	return filepath.Join(home, ".urso")
}
