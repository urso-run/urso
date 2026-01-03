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
	vector VectorObs
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
	vector VectorObs,
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
		vector: vector,
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

	defaultConfig := `# Replace this URL with the GitHub org or repo you want this runner to serve.
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
	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Warn("could not get machine hostname, using unknown", "error", err)
		hostname = "unknown"
	}

	managed, machineID, machineToken, err := c.detectManaged()
	if err != nil {
		return err
	}

	var cfg Config
	var regProvider, remProvider func() (string, error)

	if managed {
		cfg, regProvider, remProvider, err = c.loadManagedRunInputs(ctx, hostname, machineID, machineToken)
	} else {
		cfg, regProvider, remProvider, err = c.loadLocalRunInputs(configPath, registerToken, removeToken)
	}

	if err != nil {
		return err
	}

	if err := c.syncer.Sync(ctx, c.store.UrsoHome(), cfg, regProvider, remProvider); err != nil {
		return fmt.Errorf("error synchronizing runners: %w", err)
	}

	if managed && c.vector != nil {
		if err := c.vector.UpdateConfig(machineID, machineToken, cfg.Runners); err != nil {
			c.logger.Warn("failed to update vector configuration", "error", err)
		}
	}

	return nil
}

// Install handles the logic for the 'install' command.
func (c *CLI) Install(ctx context.Context, registrationToken string) error {
	c.logger.Info("starting urso service installation")

	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Warn("could not get machine hostname, using unknown", "error", err)
		hostname = "unknown"
	}

	switch {
	case err == nil:
		c.logger.Info("using existing machine credentials")
	case !errors.Is(err, ErrMissingCredentials):
		return fmt.Errorf("failed to load credentials: %w", err)
	case registrationToken == "":
		return errors.New("urso-registration-token is required for installation")
	default:
		machineID, machineToken, err := c.registerMachine(ctx, hostname, registrationToken)
		if err != nil {
			return err
		}
		c.logger.Info("saving machine credentials")
		if err := c.creds.Save(machineID, machineToken); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
	}

	if c.sm == nil {
		return ErrUnsupportedOS
	}

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	c.logger.Info("installing system service")
	if err := c.sm.Install(ctx, ServiceConfig{
		Name:           DefaultUrsoServiceName,
		ExecutablePath: executablePath,
		Arguments:      []string{"run"},
		UrsoHome:       c.store.UrsoHome(),
	}); err != nil {
		return err
	}

	if c.vector != nil {
		if err := c.vector.Install(ctx); err != nil {
			c.logger.Warn("failed to install vector service", "error", err)
		}
	}

	fmt.Fprintln(c.errOut, "launchd logs: ~/Library/Logs/com.urso-run.urso.log")
	return nil
}

// Uninstall handles the logic for the 'uninstall' command.
// This command is intended for managed mode, cleaning up the service and runners.
func (c *CLI) Uninstall(ctx context.Context) error {
	c.logger.Info("starting urso service uninstallation")

	if c.sm == nil {
		return ErrUnsupportedOS
	}

	// 1. Remove the system service (launchd/systemd)
	if err := c.sm.Uninstall(ctx, DefaultUrsoServiceName); err != nil {
		return err
	}

	// 2. Remove vector service if it exists.
	if c.vector != nil {
		if err := c.vector.Uninstall(ctx); err != nil {
			c.logger.Warn("failed to uninstall vector during urso uninstall", "error", err)
		}
	}

	// 3. Perform a final sync with an empty config to remove all existing runners.
	// This ensures runners are unregistered if tokens are available via API.
	hostname, _ := os.Hostname()
	managed, machineID, machineToken, err := c.detectManaged()

	var remProvider func() (string, error)
	if err == nil && managed {
		remProvider = func() (string, error) {
			c.logger.Info("fetching remove token from API for cleanup")
			return c.api.GetRemoveToken(ctx, hostname, machineID, machineToken)
		}
	} else {
		remProvider = func() (string, error) { return "", nil }
	}

	c.logger.Info("performing final cleanup to remove all runners")
	emptyCfg := Config{Runners: []RunnerConfig{}}
	noRegProvider := func() (string, error) { return "", nil }

	if err := c.syncer.Sync(ctx, c.store.UrsoHome(), emptyCfg, noRegProvider, remProvider); err != nil {
		c.logger.Error("failed to remove runners during uninstall", "error", err)
		fmt.Fprintf(c.errOut, "Warning: failed to remove some runners: %v\n", err)
	}

	fmt.Fprintln(c.errOut, "urso service uninstalled successfully")
	return nil
}

func (c *CLI) loadLocalRunInputs(configPath, registerToken, removeToken string) (Config, func() (string, error), func() (string, error), error) {
	cfg, err := NewConfig(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil, nil, fmt.Errorf("config file not found at %s: please run 'urso init' first", configPath)
		}
		return Config{}, nil, nil, fmt.Errorf("error loading config: %w", err)
	}

	if err := c.ensureLocalTokens(registerToken, removeToken); err != nil {
		return Config{}, nil, nil, err
	}

	return cfg, func() (string, error) { return registerToken, nil }, func() (string, error) { return removeToken, nil }, nil
}

func (c *CLI) detectManaged() (bool, string, string, error) {
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

func (c *CLI) loadManagedRunInputs(ctx context.Context, hostname, machineID, machineToken string) (Config, func() (string, error), func() (string, error), error) {
	c.logger.Info("managed mode detected: fetching runners from api")

	apiRunners, err := c.api.GetRunnerConfig(ctx, hostname, machineID, machineToken)
	if err != nil {
		return Config{}, nil, nil, fmt.Errorf("error fetching runner config: %w", err)
	}

	regProvider := func() (string, error) {
		c.logger.Info("fetching github register token from urso api")
		return c.api.GetRegisterToken(ctx, hostname, machineID, machineToken)
	}

	remProvider := func() (string, error) {
		c.logger.Info("fetching github remove token from urso api")
		return c.api.GetRemoveToken(ctx, hostname, machineID, machineToken)
	}

	return Config{Runners: apiRunners}, regProvider, remProvider, nil
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

func (c *CLI) registerMachine(ctx context.Context, hostname, registrationToken string) (string, string, error) {
	c.logger.Info("registering machine with urso api", "hostname", hostname)
	machineID, machineToken, err := c.api.RegisterMachine(ctx, registrationToken, hostname)
	if err != nil {
		return "", "", fmt.Errorf("failed to register machine: %w", err)
	}
	c.logger.Info("machine registered successfully", "machine_id", machineID)
	return machineID, machineToken, nil
}
