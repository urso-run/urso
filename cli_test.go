package urso

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Test Doubles ---

type SpyConfigStore struct {
	existsResult   bool
	readResult     []byte
	readError      error
	pathResult     string
	writeWasCalled bool
	contentWritten []byte
}

func (s *SpyConfigStore) Exists() bool {
	return s.existsResult
}

func (s *SpyConfigStore) Read() ([]byte, error) {
	return s.readResult, s.readError
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

func (s *SpySyncer) Sync(_ context.Context, cfg Config, registerToken, removeToken string) error {
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

func (s *SpyServiceManager) Install(_ context.Context, executablePath string) error {
	s.installCalled = true
	s.installPath = executablePath
	return nil
}

func (s *SpyServiceManager) Uninstall(_ context.Context) error {
	s.uninstallCalled = true
	return nil
}

type SpyAPIClient struct {
	registerMachineCalled   bool
	registerMachineJWT      string
	registerMachineHostname string
	getRunnerConfigCalled   bool
	getRegisterTokenCalled  bool
	getRemoveTokenCalled    bool

	machineID    string
	machineToken string
}

func (s *SpyAPIClient) RegisterMachine(_ context.Context, jwt, hostname string) (string, string, error) {
	s.registerMachineCalled = true
	s.registerMachineJWT = jwt
	s.registerMachineHostname = hostname
	return s.machineID, s.machineToken, nil
}
func (s *SpyAPIClient) GetRunnerConfig(_ context.Context, _, _ string) ([]RunnerConfig, error) {
	s.getRunnerConfigCalled = true
	return []RunnerConfig{{Name: "api-runner", URL: "http://example.com"}}, nil
}
func (s *SpyAPIClient) GetRegisterToken(_ context.Context, _, _ string) (string, error) {
	s.getRegisterTokenCalled = true
	return "api-gh-reg-token", nil
}
func (s *SpyAPIClient) GetRemoveToken(_ context.Context, _, _ string) (string, error) {
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
		cli := NewCLI(in, out, errOut, store, nil, nil, nil, nil, logger)
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
		cli := NewCLI(in, out, errOut, store, nil, nil, nil, nil, logger)
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
		cli := NewCLI(in, out, errOut, store, nil, nil, nil, nil, logger)
		err := cli.Init()
		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}
		if !store.writeWasCalled {
			t.Error("expected Write() to be called, but it was")
		}
	})
}

func TestCLI_Run(t *testing.T) {
	t.Run("successfully calls syncer with provided config and tokens", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		spySyncer := &SpySyncer{}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, nil, spySyncer, nil, nil, nil, logger)

		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configPath, []byte("rootDir: /tmp\nrunners: []"), 0600); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}
		err := cli.Run(context.TODO(), configPath, "reg-token", "rem-token")
		if err != nil {
			t.Fatalf("Run() returned an unexpected error: %v", err)
		}
		if !spySyncer.syncCalled {
			t.Error("expected Sync to be called, but it wasn't")
		}
	})
}

// installTestHarness is a helper struct for setting up install command tests.
type installTestHarness struct {
	cli    *CLI
	api    *SpyAPIClient
	creds  *SpyCredentialStore
	sm     *SpyServiceManager
	syncer *SpySyncer
	store  *SpyConfigStore
}

func newInstallTestHarness(t *testing.T) installTestHarness {
	t.Helper()
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	logger := slog.New(slog.DiscardHandler)

	h := installTestHarness{
		api:    &SpyAPIClient{machineID: "test-id", machineToken: "test-token"},
		creds:  &SpyCredentialStore{},
		sm:     &SpyServiceManager{},
		syncer: &SpySyncer{},
		store:  &SpyConfigStore{},
	}

	h.cli = NewCLI(in, out, errOut, h.store, h.syncer, h.sm, h.api, h.creds, logger)
	return h
}

func TestCLI_Install(t *testing.T) {
	t.Run("happy path performs all steps and merges configs", func(t *testing.T) {
		h := newInstallTestHarness(t)
		setupMockConfig(t, &h)

		err := h.cli.Install(context.TODO(), "test-jwt")

		if err != nil {
			t.Fatalf("Install() returned an unexpected error: %v", err)
		}
		assertInstallStepsExecuted(t, h)
	})

	t.Run("returns an error if init has not been run", func(t *testing.T) {
		h := newInstallTestHarness(t)
		h.store.existsResult = false // Simulate config not existing
		err := h.cli.Install(context.TODO(), "test-jwt")
		if err == nil {
			t.Fatal("expected an error but got nil")
		}
		if !strings.Contains(err.Error(), "please run 'urso init' first") {
			t.Errorf("expected error message to mention 'urso init', but got: %v", err)
		}
	})

	t.Run("returns an error if token is missing", func(t *testing.T) {
		h := newInstallTestHarness(t)
		h.store.existsResult = true
		err := h.cli.Install(context.TODO(), "")
		if err == nil {
			t.Error("expected an error when token is missing, but got nil")
		}
	})
}

func setupMockConfig(t *testing.T, h *installTestHarness) {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := []byte(`rootDir: "/local/root/dir"`)
	if err := os.WriteFile(configPath, configContent, 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	h.store.existsResult = true
	h.store.pathResult = configPath
	h.store.readResult = configContent
}

func assertInstallStepsExecuted(t *testing.T, h installTestHarness) {
	t.Helper()
	if !h.api.registerMachineCalled {
		t.Error("RegisterMachine was not called")
	}
	if h.api.registerMachineHostname == "" {
		t.Error("RegisterMachine was called with empty hostname")
	}
	if !h.creds.saveCalled {
		t.Error("Save was not called")
	}
	if !h.store.writeWasCalled {
		t.Error("expected local config to be updated, but Write was not called")
	}
	if !strings.Contains(string(h.store.contentWritten), "api-runner") {
		t.Errorf("expected local config to contain 'api-runner', but got: %s", string(h.store.contentWritten))
	}
	if !h.syncer.syncCalled {
		t.Error("Sync was not called")
	}
	if !h.sm.installCalled {
		t.Error("ServiceManager.Install was not called")
	}
}
