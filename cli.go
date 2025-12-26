package urso

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// CLI holds the logic for the urso application, decoupled from the OS via interfaces.
type CLI struct {
	in    io.Reader
	out   io.Writer
	store ConfigStore
}

// NewCLI creates a new CLI with the given I/O streams and config store.
func NewCLI(in io.Reader, out io.Writer, store ConfigStore) *CLI {
	return &CLI{in: in, out: out, store: store}
}

// Init contains the business logic for the 'init' command.
func (c *CLI) Init() error {
	if c.store.Exists() {
		fmt.Fprintf(c.out, "Config file already exists at %s. Overwrite? (y/N) ", c.store.Path())

		scanner := bufio.NewScanner(c.in)
		scanner.Scan()
		input := scanner.Text()

		if strings.TrimSpace(strings.ToLower(input)) != "y" {
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

	fmt.Fprintf(c.out, "config.yaml created successfully at %s\n", c.store.Path())
	return nil
}
