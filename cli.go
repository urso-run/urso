package urso

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// CLI holds the logic for the urso application, decoupled from the OS via interfaces.
type CLI struct {
	in     io.Reader
	out    io.Writer
	store  ConfigStore
	logger *slog.Logger
}

// NewCLI creates a new CLI with the given dependencies.
func NewCLI(in io.Reader, out io.Writer, store ConfigStore, logger *slog.Logger) *CLI {
	return &CLI{in: in, out: out, store: store, logger: logger}
}

// Init contains the business logic for the 'init' command.
func (c *CLI) Init() error {
	if c.store.Exists() {
		fmt.Fprintf(c.out, "Config file already exists at %s. Overwrite? (y/N) ", c.store.Path())

		scanner := bufio.NewScanner(c.in)
		scanner.Scan()
		input := scanner.Text()

		if strings.TrimSpace(strings.ToLower(input)) != "y" {
			c.logger.Info("user aborted config file creation")
			fmt.Fprintln(c.out, "Aborted.")
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
	fmt.Fprintf(c.out, "config.yaml created successfully at %s\n", c.store.Path())
	return nil
}
