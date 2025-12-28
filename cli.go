package urso

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// CLI holds the logic for the urso application, decoupled from the OS via interfaces.
type CLI struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
	store  ConfigStore
	syncer Syncer
	sm     ServiceManager
	api    APIClient
	creds  CredentialStore
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
	sm ServiceManager,
	api APIClient,
	creds CredentialStore,
	logger *slog.Logger,
	version, commit, date string,
) *CLI {
	return &CLI{
		in:     in,
		out:    out,
		errOut: errOut,
		store:  store,
		syncer: syncer,
		sm:     sm,
		api:    api,
		creds:  creds,
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
func (c *CLI) Run(ctx context.Context, configPath, registerToken, removeToken string) error {
	cfg, err := NewConfig(configPath)
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Note: Token resolution is handled in main.go before calling this method.
	if err := c.syncer.Sync(ctx, cfg, registerToken, removeToken); err != nil {
		return fmt.Errorf("error synchronizing runners: %w", err)
	}
	return nil
}

// Install handles the logic for the 'install' command.
func (c *CLI) Install(ctx context.Context, registrationToken string) error {
	c.logger.Info("starting urso service installation")

	if !c.store.Exists() {
		return fmt.Errorf("config file not found at %s, please run 'urso init' first", c.store.Path())
	}
	if registrationToken == "" {
		return errors.New("urso-registration-token is required for installation")
	}

	// 1. Register machine with Urso API
	machineID, machineToken, err := c.registerMachine(ctx, registrationToken)
	if err != nil {
		return err
	}

	// 2. Save credentials
	c.logger.Info("saving machine credentials")
	if err := c.creds.Save(machineID, machineToken); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	// 3. Load local config to get rootDir
	localConfig, err := NewConfig(c.store.Path())
	if err != nil {
		return fmt.Errorf("failed to load local config: %w", err)
	}

	// 4. Fetch the runner config from the API
	c.logger.Info("fetching runner config from urso api")
	apiRunners, err := c.api.GetRunnerConfig(ctx, machineID, machineToken)
	if err != nil {
		return fmt.Errorf("failed to get runner config: %w", err)
	}

	// 5. Update local config with runners from API
	c.logger.Info("updating local config with api runners")
	if err := c.updateLocalConfig(apiRunners); err != nil {
		return fmt.Errorf("failed to update local config: %w", err)
	}

	// 6. Merge local rootDir with API runners
	finalConfig := Config{
		RootDir: localConfig.RootDir,
		Runners: apiRunners,
	}

	// 7. Fetch GitHub tokens from API
	c.logger.Info("fetching github tokens from urso api")
	ghRegisterToken, err := c.api.GetRegisterToken(ctx, machineID, machineToken)
	if err != nil {
		return fmt.Errorf("failed to get github register token: %w", err)
	}
	ghRemoveToken, err := c.api.GetRemoveToken(ctx, machineID, machineToken)
	if err != nil {
		return fmt.Errorf("failed to get github remove token: %w", err)
	}

	// 8. Run the synchronization logic
	c.logger.Info("performing initial runner synchronization")
	if err := c.syncer.Sync(ctx, finalConfig, ghRegisterToken, ghRemoveToken); err != nil {
		return fmt.Errorf("failed to sync runners: %w", err)
	}

	// 9. Install the service
	if c.sm == nil {
		return ErrUnsupportedOS
	}

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	c.logger.Info("installing system service")
	return c.sm.Install(ctx, executablePath)
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

func (c *CLI) updateLocalConfig(runners []RunnerConfig) error {
	data, err := c.store.Read()
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("could not unmarshal config: %w", err)
	}

	cfg.Runners = runners

	updated, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal updated config: %w", err)
	}

	if err := c.store.Write(updated); err != nil {
		return fmt.Errorf("could not write updated config: %w", err)
	}

	return nil
}

func (c *CLI) registerMachine(ctx context.Context, registrationToken string) (string, string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Warn("could not get machine hostname, using unknown", "error", err)
	}
	c.logger.Info("registering machine with urso api", "hostname", hostname)
	machineID, machineToken, err := c.api.RegisterMachine(ctx, registrationToken, hostname)
	if err != nil {
		return "", "", fmt.Errorf("failed to register machine: %w", err)
	}
	c.logger.Info("machine registered successfully", "machine_id", machineID)
	return machineID, machineToken, nil
}
