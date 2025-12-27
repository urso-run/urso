package urso

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// CLI holds the logic for the urso application, decoupled from the OS via interfaces.
type CLI struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
	store  ConfigStore
	syncer Syncer
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
	syncer Syncer,
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

// Init handles the logic for the 'init' command.
func (c *CLI) Init() error {
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

// Run executes the main sync logic using the provided configuration and tokens.
func (c *CLI) Run(configPath, registerToken, removeToken string) error {
	cfg, err := NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Note: Token resolution is handled in main.go before calling this method.
	if err := c.syncer.Sync(cfg, registerToken, removeToken); err != nil {
		return fmt.Errorf("error synchronizing runners: %w", err)
	}
	return nil
}

// Install handles the logic for the 'install' command.
func (c *CLI) Install(registrationToken string) error {
	c.logger.Info("The install command is a paid feature and is not yet implemented.")
	c.logger.Info("Thank you for your interest!")
	if registrationToken == "" {
		return errors.New("urso-registration-token is required for installation")
	}
	return nil
}

// Version prints the application's version information.
func (c *CLI) Version() {
	fmt.Fprintf(c.out, "urso version %s, commit %s, built at %s\n", c.version, c.commit, c.date)
}

// PrintUsage prints the command-line usage information.
func (c *CLI) PrintUsage() {
	fmt.Fprintf(c.errOut, `Usage: urso <command> [--github-register-token <token> | --github-remove-token <token> | --urso-registration-token <token>]

Available commands:
  init      Create a default config.yaml for runners
  run       Run the sync to create/remove runners based on config.yaml
  install   Install urso as a service (paid license only)
  version   Print the version number
  help      Show this help message

Parameters:
  --github-register-token     github actions runner registration token
  --github-remove-token       github actions runner remove token
  --urso-registration-token   urso registration token (obtained with a license)

`)
}
