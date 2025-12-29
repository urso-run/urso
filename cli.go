package urso

import (
	"bufio"
	"bytes"
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
# Replace this URL with the GitHub org or repo you want this runner to serve.
runners:
  - name: "default-runner"
    url: "https://github.com/your-org"
    group: "Default"
    labels:
      - self-hosted
      - macos
      - arm64
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

	licensed, machineID, machineToken, err := c.detectLicensed()
	if err != nil {
		return err
	}

	if licensed {
		cfg, registerToken, removeToken, err = c.loadLicensedRunInputs(ctx, cfg, machineID, machineToken)
		if err != nil {
			return err
		}
	} else {
		if err := c.ensureLocalTokens(registerToken, removeToken); err != nil {
			return err
		}
	}

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

	machineID, machineToken, err := c.creds.Load()
	switch {
	case err == nil:
		c.logger.Info("using existing machine credentials")
	case !errors.Is(err, ErrMissingCredentials):
		return fmt.Errorf("failed to load credentials: %w", err)
	case registrationToken == "":
		return errors.New("urso-registration-token is required for installation")
	default:
		machineID, machineToken, err = c.registerMachine(ctx, registrationToken)
		if err != nil {
			return err
		}
		c.logger.Info("saving machine credentials")
		if err := c.creds.Save(machineID, machineToken); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
	}

	if err := c.performInitialSync(ctx, machineID, machineToken); err != nil {
		return err
	}

	if c.sm == nil {
		return ErrUnsupportedOS
	}

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	c.logger.Info("installing system service")
	if err := c.sm.Install(ctx, executablePath); err != nil {
		return err
	}
	fmt.Fprintln(c.errOut, "launchd logs: ~/Library/Logs/com.repeat.urso.log")
	return nil
}

func (c *CLI) detectLicensed() (bool, string, string, error) {
	if c.creds == nil || c.api == nil {
		return false, "", "", nil
	}

	id, token, err := c.creds.Load()
	if err == nil {
		return true, id, token, nil
	}
	if errors.Is(err, ErrMissingCredentials) {
		return false, "", "", nil
	}
	return false, "", "", fmt.Errorf("error loading credentials: %w", err)
}

func (c *CLI) loadLicensedRunInputs(ctx context.Context, cfg Config, machineID, machineToken string) (Config, string, string, error) {
	c.logger.Info("licensed mode detected: fetching runners and tokens from api")

	apiRunners, err := c.api.GetRunnerConfig(ctx, machineID, machineToken)
	if err != nil {
		return Config{}, "", "", fmt.Errorf("error fetching runner config: %w", err)
	}
	cfg.Runners = apiRunners

	registerToken, err := c.api.GetRegisterToken(ctx, machineID, machineToken)
	if err != nil {
		return Config{}, "", "", fmt.Errorf("error fetching github register token: %w", err)
	}
	removeToken, err := c.api.GetRemoveToken(ctx, machineID, machineToken)
	if err != nil {
		return Config{}, "", "", fmt.Errorf("error fetching github remove token: %w", err)
	}

	return cfg, registerToken, removeToken, nil
}

func (c *CLI) ensureLocalTokens(registerToken, removeToken string) error {
	if registerToken == "" {
		return errors.New("github-register-token is required in local mode")
	}
	if removeToken == "" {
		return errors.New("github-remove-token is required in local mode")
	}
	return nil
}

func (c *CLI) performInitialSync(ctx context.Context, id, token string) error {
	localConfig, err := c.loadLocalConfig()
	if err != nil {
		return err
	}

	// Fetch the runner config from the API
	c.logger.Info("fetching runner config from urso api")
	apiRunners, err := c.api.GetRunnerConfig(ctx, id, token)
	if err != nil {
		return fmt.Errorf("failed to get runner config: %w", err)
	}

	// Update local config with runners from API
	c.logger.Info("updating local config with api runners")
	if err := c.updateLocalConfig(apiRunners); err != nil {
		return fmt.Errorf("failed to update local config: %w", err)
	}

	// Fetch GitHub tokens from API
	c.logger.Info("fetching github tokens from urso api")
	ghRegisterToken, err := c.api.GetRegisterToken(ctx, id, token)
	if err != nil {
		return fmt.Errorf("failed to get github register token: %w", err)
	}
	ghRemoveToken, err := c.api.GetRemoveToken(ctx, id, token)
	if err != nil {
		return fmt.Errorf("failed to get github remove token: %w", err)
	}

	finalConfig := Config{
		RootDir: localConfig.RootDir,
		Runners: apiRunners,
	}

	// Run the synchronization logic
	c.logger.Info("performing initial runner synchronization")
	if err := c.syncer.Sync(ctx, finalConfig, ghRegisterToken, ghRemoveToken); err != nil {
		return fmt.Errorf("failed to sync runners: %w", err)
	}

	return nil
}

func (c *CLI) updateLocalConfig(runners []RunnerConfig) error {
	cfg, err := c.loadLocalConfig()
	if err != nil {
		return err
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
		hostname = "unknown"
	}
	c.logger.Info("registering machine with urso api", "hostname", hostname)
	machineID, machineToken, err := c.api.RegisterMachine(ctx, registrationToken, hostname)
	if err != nil {
		return "", "", fmt.Errorf("failed to register machine: %w", err)
	}
	c.logger.Info("machine registered successfully", "machine_id", machineID)
	return machineID, machineToken, nil
}

func (c *CLI) loadLocalConfig() (Config, error) {
	data, err := c.store.Read()
	if err != nil {
		return Config{}, fmt.Errorf("failed to read local config: %w", err)
	}

	cfg, err := ParseConfig(bytes.NewReader(data))
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse local config: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("could not get home directory: %w", err)
	}
	cfg.ExpandPaths(home)

	return cfg, nil
}
