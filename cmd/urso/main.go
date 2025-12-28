package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/repeat-dev/urso"
	"github.com/spf13/cobra"
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
		configPath    string
		registerToken string
		removeToken   string
		installToken  string
	)

	rootCmd := &cobra.Command{
		Use:     "urso",
		Short:   "Urso is a GitHub Actions runner manager",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	}

	// Persistent flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&configPath, "config", defaultConfigPath(), "path to the configuration file")

	// Standard streams
	out := os.Stdout
	errOut := os.Stderr
	in := os.Stdin

	// Dependencies
	logger := slog.New(slog.NewTextHandler(errOut, nil))
	store, err := urso.NewFileSystemConfigStore()
	if err != nil {
		// We handle this during execution if needed, but logging it here for now
		logger.Error("failed to initialize configuration store", "error", err)
	}
	httpClient := urso.NewHTTPClient()
	apiClient := &urso.DashboardAPIClient{
		BaseURL:    "https://urso.run",
		HTTPClient: httpClient,
		Logger:     logger,
	}
	credStore, _ := urso.NewFileSystemCredentialStore()
	syncer := urso.NewRunnerSyncer(
		&urso.FileSystemMachine{},
		urso.NewGithubAPIDownloader(httpClient),
		urso.NewLiveRunnerExecutor(out),
		logger,
	)
	sm, _ := urso.NewServiceManager(logger)

	cli := urso.NewCLI(in, out, errOut, store, syncer, sm, apiClient, credStore, logger)

	// Command: init
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Create a default config.yaml for runners",
		RunE: func(_ *cobra.Command, _ []string) error {
			return cli.Init()
		},
	}

	// Command: run
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the sync to create/remove runners based on config.yaml",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), commandTimeout)
			defer cancel()

			reg := urso.ResolveToken(registerToken, urso.EnvVarRegisterToken)
			rem := urso.ResolveToken(removeToken, urso.EnvVarRemoveToken)

			return cli.Run(ctx, configPath, reg, rem)
		},
	}
	runCmd.Flags().StringVar(&registerToken, "github-register-token", "", "token to register github actions runner")
	runCmd.Flags().StringVar(&removeToken, "github-remove-token", "", "token to remove github actions runner")

	// Command: install
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install urso as a service (paid license only)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), commandTimeout)
			defer cancel()

			return cli.Install(ctx, installToken)
		},
	}
	installCmd.Flags().StringVar(&installToken, "urso-registration-token", "", "urso registration token")

	rootCmd.AddCommand(initCmd, runCmd, installCmd)

	return rootCmd
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".urso", "config.yaml")
}
