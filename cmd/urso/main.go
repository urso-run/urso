package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/repeat-dev/urso"
)

// These variables are set at build time via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// 1. Create the top-level logger.
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// 2. Create all the "real" dependencies for the application.
	// These are the live implementations of our interfaces that interact
	// with the filesystem, network, and shell.
	store, err := urso.NewFileSystemConfigStore()
	if err != nil {
		logger.Error("failed to initialize configuration store", "error", err)
		os.Exit(1)
	}

	machine := &urso.FileSystemMachine{}
	downloader := &urso.GithubAPIDownloader{}
	// Command output from the runner scripts will be written to standard out.
	executor := urso.NewLiveRunnerExecutor(os.Stdout)
	syncer := urso.NewRunnerSyncer(machine, downloader, executor, logger)

	// 3. Create the CLI application itself, injecting all the dependencies.
	cli := urso.NewCLI(
		os.Stdin,
		os.Stdout,
		os.Stderr,
		store,
		syncer,
		logger,
		version,
		commit,
		date,
	)

	// 4. Execute the application logic.
	if err := cli.Execute(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
