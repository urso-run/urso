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

type SpyAPIClient struct {
	registerMachineCalled  bool
	registerMachineJWT     string
	getRunnerConfigCalled  bool
	getRegisterTokenCalled bool
	getRemoveTokenCalled   bool

	machineID    string
	machineToken string
}

func (s *SpyAPIClient) RegisterMachine(jwt string) (string, string, error) {
	s.registerMachineCalled = true
	s.registerMachineJWT = jwt
	return s.machineID, s.machineToken, nil
}
func (s *SpyAPIClient) GetRunnerConfig(_, _ string) (Config, error) {
	s.getRunnerConfigCalled = true
	return Config{Runners: []RunnerConfig{{Name: "api-runner"}}}, nil
}
func (s *SpyAPIClient) GetRegisterToken(_, _ string) (string, error) {
	s.getRegisterTokenCalled = true
	return "api-gh-reg-token", nil
}
func (s *SpyAPIClient) GetRemoveToken(_, _ string) (string, error) {
	s.getRemoveTokenCalled = true
	return "api-gh-rem-token", nil
}

type SpyCredentialStore struct {
	saveCalled bool
	loadCalled bool
	savedID    string
	savedToken string
}

func (s *SpyCredentialStore) Save(id, token string) error {
	s.saveCalled = true
	s.savedID = id
	s.savedToken = token
	return nil
}
func (s *SpyCredentialStore) Load() (string, string, error) {
	s.loadCalled = true
	return "loaded-id", "loaded-token", nil
}

// --- Tests ---

func TestCLI_Init(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("creates config file if it does not exist", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		store := &SpyConfigStore{existsResult: false, pathResult: "/test/config.yaml"}
		cli := NewCLI(in, out, errOut, store, nil, nil, nil, nil, logger, "", "", "")
		err := cli.Init()
		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}
		if !store.writeWasCalled {
			t.Error("expected Write() to be called, but it wasn't")
		}
	})

	t.Run("aborts if config file exists and user says no", func(t *testing.T) {
		in, out, errOut := strings.NewReader("n\n"), &bytes.Buffer{}, &bytes.Buffer{}
		store := &SpyConfigStore{existsResult: true, pathResult: "/test/config.yaml"}
		cli := NewCLI(in, out, errOut, store, nil, nil, nil, nil, logger, "", "", "")
		err := cli.Init()
		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}
		if store.writeWasCalled {
			t.Error("expected Write() not to be called, but it was")
		}
	})

	t.Run("overwrites if config file exists and user says yes", func(t *testing.T) {
		in, out, errOut := strings.NewReader("y\n"), &bytes.Buffer{}, &bytes.Buffer{}
		store := &SpyConfigStore{existsResult: true, pathResult: "/test/config.yaml"}
		cli := NewCLI(in, out, errOut, store, nil, nil, nil, nil, logger, "", "", "")
		err := cli.Init()
		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}
		if !store.writeWasCalled {
			t.Error("expected Write() to be called, but it wasn't")
		}
	})
}

func TestCLI_Run(t *testing.T) {
	t.Run("successfully calls syncer with provided config and tokens", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		spySyncer := &SpySyncer{}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, nil, spySyncer, nil, nil, nil, logger, "", "", "")

		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte("runners: []"), 0600); err != nil {
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
	})
}

func TestCLI_Install(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("happy path: performs all installation steps in order", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		spyAPI := &SpyAPIClient{machineID: "test-id", machineToken: "test-token"}
		spyCreds := &SpyCredentialStore{}
		spySM := &SpyServiceManager{}
		spySyncer := &SpySyncer{}
		cli := NewCLI(in, out, errOut, nil, spySyncer, spySM, spyAPI, spyCreds, logger, "", "", "")

		err := cli.Install("test-jwt")
		if err != nil {
			t.Fatalf("Install() returned an unexpected error: %v", err)
		}
		if !spyAPI.registerMachineCalled {
			t.Error("RegisterMachine was not called")
		}
		if !spyCreds.saveCalled {
			t.Error("Save was not called")
		}
		if !spySyncer.syncCalled {
			t.Error("Sync was not called")
		}
		if !spySM.installCalled {
			t.Error("ServiceManager.Install was not called")
		}
	})

	t.Run("returns an error if token is missing", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		cli := NewCLI(in, out, errOut, nil, nil, nil, nil, nil, logger, "", "", "")
		err := cli.Install("")
		if err == nil {
			t.Error("expected an error when token is missing, but got nil")
		}
	})

	t.Run("returns an error for an unsupported OS", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		spyAPI := &SpyAPIClient{}
		spyCreds := &SpyCredentialStore{}
		spySyncer := &SpySyncer{}
		cli := NewCLI(in, out, errOut, nil, spySyncer, nil, spyAPI, spyCreds, logger, "", "", "") // nil ServiceManager
		err := cli.Install("some-token")
		if !errors.Is(err, ErrUnsupportedOS) {
			t.Errorf("got error %v, want %v", err, ErrUnsupportedOS)
		}
	})
}

func TestCLI_Version(t *testing.T) {
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	logger := slog.New(slog.DiscardHandler)
	cli := NewCLI(in, out, errOut, nil, nil, nil, nil, nil, logger, "1.2.3", "abc", "date")
	cli.Version()
	expected := "urso version 1.2.3, commit abc, built at date\n"
	if out.String() != expected {
		t.Errorf("got %q, want %q", out.String(), expected)
	}
}
