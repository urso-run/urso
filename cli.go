package urso

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// CLI holds the logic for the urso application, decoupled from the OS via interfaces.
type CLI struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
	store  ConfigStore
	syncer *RunnerSyncer
	logger *slog.Logger

	version string
	commit  string
	date    string
}

// NewCLI creates a new CLI with the given dependencies.
func NewCLI(
	in io.Reader,
	out io.Writer,
	errOut io.Writer,
	store ConfigStore,
	syncer *RunnerSyncer,
	logger *slog.Logger,
	version, commit, date string,
) *CLI {
	return &CLI{
		in:     in,
		out:    out,
		errOut: errOut,
		store:  store,
		syncer: syncer,
		logger: logger,

		version: version,
		commit:  commit,
		date:    date,
	}
}

// Execute is the main entrypoint for the CLI logic. It parses arguments and
// runs the appropriate command.
func (c *CLI) Execute(args []string) error {
	const minArgs = 2
	if len(args) < minArgs {
		c.printUsage()
		return errors.New("expected 'init', 'run', 'install', 'version' or 'help' subcommands")
	}

	command := args[1]
	handlerArgs := args[2:]

	switch command {
	case "init":
		return c.Init(handlerArgs)
	case "run":
		return c.Run(handlerArgs)
	case "install":
		return c.Install(handlerArgs)
	case "version":
		c.printVersion()
		return nil
	case "help":
		c.printUsage()
		return nil
	default:
		fmt.Fprintf(c.errOut, "unknown command: %s\n\n", command)
		c.printUsage()
		return fmt.Errorf("unknown command: %s", command)
	}
}

// Init handles the logic for the 'init' command, including flag parsing.
func (c *CLI) Init(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(c.errOut)
	fs.Usage = func() {
		fmt.Fprintln(c.errOut, "Usage: urso init")
		fmt.Fprintln(c.errOut, "Creates a default config.yaml in ~/.urso/config.yaml")
	}
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	if c.store.Exists() {
		fmt.Fprintf(c.errOut, "Config file already exists at %s. Overwrite? (y/N) ", c.store.Path())

		scanner := bufio.NewScanner(c.in)
		scanner.Scan()
		input := scanner.Text()

		if strings.TrimSpace(strings.ToLower(input)) != "y" {
			c.logger.Info("user aborted config file creation")
			fmt.Fprintln(c.errOut, "Aborted.")
			return nil
		}
	}

	defaultConfig := `rootDir: ".urso/runners"
runners:
  - name: "default-runner"
    labels:
      - self-hosted
      - linux
      - x64
    # url: "https://github.com/my-org"
`
	err := c.store.Write([]byte(defaultConfig))
	if err != nil {
		return err
	}

	c.logger.Info("config file created", "path", c.store.Path())
	fmt.Fprintf(c.errOut, "config.yaml created successfully at %s\n", c.store.Path())
	return nil
}

// Run handles the logic for the 'run' command, including flag parsing.
func (c *CLI) Run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(c.errOut)
	fs.Usage = func() {
		fmt.Fprintln(c.errOut, "Usage: urso run [options]")
		fmt.Fprintln(c.errOut, "Synchronizes runners based on the config file.")
		fmt.Fprintln(c.errOut, "\nOptions:")
		fs.PrintDefaults()
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not get user home directory: %w", err)
	}
	defaultConfigPath := filepath.Join(home, ".urso", "config.yaml")

	configPath := fs.String("config", defaultConfigPath, "path to the configuration file")
	registerToken := fs.String("github-register-token", "", "token to register github actions runner")
	removeToken := fs.String("github-remove-token", "", "token to remove github actions runner")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags for run command: %w", err)
	}

	cfg, err := NewConfig(*configPath)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	regToken := ResolveToken(*registerToken, EnvVarRegisterToken)
	remToken := ResolveToken(*removeToken, EnvVarRemoveToken)

	return c.syncer.Sync(cfg, regToken, remToken)
}

// Install handles the logic for the 'install' command, including flag parsing.
func (c *CLI) Install(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(c.errOut)
	fs.Usage = func() {
		fmt.Fprintln(c.errOut, "Usage: urso install [options]")
		fmt.Fprintln(c.errOut, "Installs urso as a service (requires a paid license).")
		fmt.Fprintln(c.errOut, "\nOptions:")
		fs.PrintDefaults()
	}
	registrationToken := fs.String("urso-registration-token", "", "urso registration token")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("failed to parse flags for install command: %w", err)
	}

	c.logger.Info("The install command is a paid feature and is not yet implemented.")
	c.logger.Info("Thank you for your interest!")

	if *registrationToken == "" {
		return errors.New("urso-registration-token is required for installation")
	}

	return nil
}

func (c *CLI) printVersion() {
	fmt.Fprintf(c.out, "urso version %s, commit %s, built at %s\n", c.version, c.commit, c.date)
}

func (c *CLI) printUsage() {
	fmt.Fprintf(c.errOut, `Usage: urso <command> [arguments]

Available commands:
  init      Create a default config.yaml for runners
  run       Run the sync to create/remove runners based on config.yaml
  install   Install urso as a service (paid license only)
  version   Print the version number
  help      Show this help message
`)
}
