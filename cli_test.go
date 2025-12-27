package urso

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Test Doubles ---

type SpyConfigStore struct {
	existsResult   bool
	pathResult     string
	writeWasCalled bool
	contentWritten []byte
}

func (s *SpyConfigStore) Exists() bool {
	return s.existsResult
}

func (s *SpyConfigStore) Write(content []byte) error {
	s.writeWasCalled = true
	s.contentWritten = content
	return nil
}

func (s *SpyConfigStore) Path() string {
	return s.pathResult
}

// SpySyncer is a test double for the Syncer interface.
type SpySyncer struct {
	syncCalled        bool
	syncCfg           Config
	syncRegisterToken string
	syncRemoveToken   string
}

func (s *SpySyncer) Sync(cfg Config, registerToken, removeToken string) error {
	s.syncCalled = true
	s.syncCfg = cfg
	s.syncRegisterToken = registerToken
	s.syncRemoveToken = removeToken
	return nil
}

// SpyServiceManager is a test double for the ServiceManager interface.
type SpyServiceManager struct {
	installCalled   bool
	uninstallCalled bool
	installPath     string
}

func (s *SpyServiceManager) Install(executablePath string) error {
	s.installCalled = true
	s.installPath = executablePath
	return nil
}

func (s *SpyServiceManager) Uninstall() error {
	s.uninstallCalled = true
	return nil
}

// --- Tests ---

func TestCLI_Init(t *testing.T) {
	t.Run("creates config file if it does not exist", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		store := &SpyConfigStore{existsResult: false, pathResult: "/test/config.yaml"}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, store, nil, nil, logger, "", "", "")

		err := cli.Init()

		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}
		if !store.writeWasCalled {
			t.Error("expected Write() to be called, but it wasn't")
		}
		expectedOutput := "config.yaml created successfully"
		if !strings.Contains(errOut.String(), expectedOutput) {
			t.Errorf("expected output to contain %q, but got %q", expectedOutput, errOut.String())
		}
	})

	t.Run("aborts if config file exists and user says no", func(t *testing.T) {
		in, out, errOut := strings.NewReader("n\n"), &bytes.Buffer{}, &bytes.Buffer{}
		store := &SpyConfigStore{existsResult: true, pathResult: "/test/config.yaml"}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, store, nil, nil, logger, "", "", "")

		err := cli.Init()

		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}
		if store.writeWasCalled {
			t.Error("expected Write() not to be called, but it was")
		}
		expectedPrompt := "Overwrite? (y/N)"
		if !strings.Contains(errOut.String(), expectedPrompt) {
			t.Errorf("expected output to contain prompt %q, but got %q", expectedPrompt, errOut.String())
		}
	})

	t.Run("overwrites if config file exists and user says yes", func(t *testing.T) {
		in, out, errOut := strings.NewReader("y\n"), &bytes.Buffer{}, &bytes.Buffer{}
		store := &SpyConfigStore{existsResult: true, pathResult: "/test/config.yaml"}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, store, nil, nil, logger, "", "", "")

		err := cli.Init()

		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}
		if !store.writeWasCalled {
			t.Error("expected Write() to be called, but it wasn't")
		}
		expectedOutput := "config.yaml created successfully"
		if !strings.Contains(errOut.String(), expectedOutput) {
			t.Errorf("expected output to contain %q, but got %q", expectedOutput, errOut.String())
		}
	})
}

func TestCLI_Run(t *testing.T) {
	t.Run("successfully calls syncer with provided config and tokens", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		store := &SpyConfigStore{} // Not used by Run, but needed for constructor
		spySyncer := &SpySyncer{}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, store, spySyncer, nil, logger, "dev", "test", "now")

		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		configContent := `
rootDir: ".urso/runners"
runners:
  - name: "test-runner"
`
		if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		err := cli.Run(configPath, "reg-token", "rem-token")

		if err != nil {
			t.Fatalf("Run() returned an unexpected error: %v", err)
		}

		if !spySyncer.syncCalled {
			t.Error("expected Sync to be called, but it wasn't")
		}
		if spySyncer.syncRegisterToken != "reg-token" {
			t.Errorf("got register token %q, want 'reg-token'", spySyncer.syncRegisterToken)
		}
		if spySyncer.syncRemoveToken != "rem-token" {
			t.Errorf("got remove token %q, want 'rem-token'", spySyncer.syncRemoveToken)
		}
		if len(spySyncer.syncCfg.Runners) != 1 || spySyncer.syncCfg.Runners[0].Name != "test-runner" {
			t.Errorf("Sync was called with incorrect config: %+v", spySyncer.syncCfg)
		}
	})
}

func TestCLI_Install(t *testing.T) {
	t.Run("calls the service manager's install method", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		spySM := &SpyServiceManager{}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, nil, nil, spySM, logger, "dev", "test", "now")

		// The token is passed but not used by this part of the logic yet.
		err := cli.Install("some-token")

		if err != nil {
			t.Fatalf("Install() returned an unexpected error: %v", err)
		}

		if !spySM.installCalled {
			t.Error("expected ServiceManager.Install to be called, but it wasn't")
		}

		// Check that it tried to install the current executable
		execPath, _ := os.Executable()
		if spySM.installPath != execPath {
			t.Errorf("got executable path %q, want %q", spySM.installPath, execPath)
		}
	})

	t.Run("returns an error for an unsupported OS", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		logger := slog.New(slog.DiscardHandler)
		// Pass nil for the ServiceManager to simulate an unsupported OS
		cli := NewCLI(in, out, errOut, nil, nil, nil, logger, "dev", "test", "now")

		err := cli.Install("some-token")

		if err == nil {
			t.Fatal("expected an error for unsupported OS, but got nil")
		}
		if !errors.Is(err, ErrUnsupportedOS) {
			t.Errorf("got error %v, want %v", err, ErrUnsupportedOS)
		}
	})

	t.Run("returns an error if token is missing", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		spySM := &SpyServiceManager{}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, nil, nil, spySM, logger, "dev", "test", "now")

		err := cli.Install("")

		if err == nil {
			t.Error("expected an error when token is missing, but got nil")
		}
	})
}

func TestCLI_Version(t *testing.T) {
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	logger := slog.New(slog.DiscardHandler)
	cli := NewCLI(in, out, errOut, nil, nil, nil, logger, "1.2.3", "abc1234", "2024-01-01")

	cli.Version()

	expected := "urso version 1.2.3, commit abc1234, built at 2024-01-01\n"
	if out.String() != expected {
		t.Errorf("got %q, want %q", out.String(), expected)
	}
}
