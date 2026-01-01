package urso

import (
	"bytes"
	"context"
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
	readResult     []byte
	readError      error
	pathResult     string
	homeResult     string
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

func (s *SpyConfigStore) UrsoHome() string {
	return s.homeResult
}

type SpySyncer struct {
	syncCalled        bool
	syncRootDir       string
	syncCfg           Config
	syncRegisterToken string
	syncRemoveToken   string
}

func (s *SpySyncer) Sync(_ context.Context, rootDir string, cfg Config, registerToken, removeToken string) error {
	s.syncCalled = true
	s.syncRootDir = rootDir
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

	machineID           string
	machineToken        string
	runnerConfigs       []RunnerConfig
	registerToken       string
	removeToken         string
	getRunnerConfigErr  error
	getRegisterTokenErr error
	getRemoveTokenErr   error
}

func (s *SpyAPIClient) RegisterMachine(_ context.Context, jwt, hostname string) (string, string, error) {
	s.registerMachineCalled = true
	s.registerMachineJWT = jwt
	s.registerMachineHostname = hostname
	return s.machineID, s.machineToken, nil
}
func (s *SpyAPIClient) GetRunnerConfig(_ context.Context, _, _, _ string) ([]RunnerConfig, error) {
	s.getRunnerConfigCalled = true
	if s.getRunnerConfigErr != nil {
		return nil, s.getRunnerConfigErr
	}
	if s.runnerConfigs != nil {
		return s.runnerConfigs, nil
	}
	return []RunnerConfig{{Name: "api-runner", URL: "http://example.com"}}, nil
}
func (s *SpyAPIClient) GetRegisterToken(_ context.Context, _, _, _ string) (string, error) {
	s.getRegisterTokenCalled = true
	if s.getRegisterTokenErr != nil {
		return "", s.getRegisterTokenErr
	}
	if s.registerToken != "" {
		return s.registerToken, nil
	}
	return "api-gh-reg-token", nil
}
func (s *SpyAPIClient) GetRemoveToken(_ context.Context, _, _, _ string) (string, error) {
	s.getRemoveTokenCalled = true
	if s.getRemoveTokenErr != nil {
		return "", s.getRemoveTokenErr
	}
	if s.removeToken != "" {
		return s.removeToken, nil
	}
	return "api-gh-rem-token", nil
}

type SpyCredentialStore struct {
	saveCalled bool
	loadCalled bool
	savedID    string
	savedToken string
	loadID     string
	loadToken  string
	loadErr    error
}

func (s *SpyCredentialStore) Save(id, token string) error {
	s.saveCalled = true
	s.savedID = id
	s.savedToken = token
	return nil
}
func (s *SpyCredentialStore) Load() (string, string, error) {
	s.loadCalled = true
	if s.loadErr != nil {
		return s.loadID, s.loadToken, s.loadErr
	}
	if s.loadID == "" && s.loadToken == "" {
		return "", "", ErrMissingCredentials
	}
	return s.loadID, s.loadToken, nil
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

func TestCLI_Run_LocalSuccess(t *testing.T) {
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	spySyncer := &SpySyncer{}
	logger := slog.New(slog.DiscardHandler)

	tmpDir := t.TempDir()
	store := &SpyConfigStore{homeResult: tmpDir}
	cli := NewCLI(in, out, errOut, store, spySyncer, nil, nil, nil, logger)

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("runners: []"), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
	err := cli.Run(context.TODO(), configPath, "reg-token", "rem-token")
	if err != nil {
		t.Fatalf("Run() returned an unexpected error: %v", err)
	}
	if !spySyncer.syncCalled {
		t.Error("expected Sync to be called, but it wasn't")
	}
	if spySyncer.syncRegisterToken != "reg-token" {
		t.Errorf("expected register token to be forwarded, got %s", spySyncer.syncRegisterToken)
	}
	if spySyncer.syncRemoveToken != "rem-token" {
		t.Errorf("expected remove token to be forwarded, got %s", spySyncer.syncRemoveToken)
	}
}

func TestCLI_Run_LocalRequiresTokens(t *testing.T) {
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	spySyncer := &SpySyncer{}
	logger := slog.New(slog.DiscardHandler)

	tmpDir := t.TempDir()
	store := &SpyConfigStore{homeResult: tmpDir}
	cli := NewCLI(in, out, errOut, store, spySyncer, nil, nil, nil, logger)

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("runners: []"), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	err := cli.Run(context.TODO(), configPath, "", "rem-token")
	if err == nil {
		t.Fatal("expected an error for missing register token, got nil")
	}
	if !strings.Contains(err.Error(), "github-register-token") {
		t.Fatalf("expected github-register-token error, got %v", err)
	}
	if spySyncer.syncCalled {
		t.Fatal("expected syncer not to be called in error path")
	}
}

func TestCLI_Run_ManagedFetchesFromAPI(t *testing.T) {
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	spySyncer := &SpySyncer{}
	logger := slog.New(slog.DiscardHandler)
	spyAPI := &SpyAPIClient{
		machineID:     "mid",
		machineToken:  "mtok",
		runnerConfigs: []RunnerConfig{{Name: "api-runner", URL: "https://github.com/org"}},
		registerToken: "api-reg-token",
		removeToken:   "api-rem-token",
	}
	spyCreds := &SpyCredentialStore{
		loadID:    "mid",
		loadToken: "mtok",
	}
	tmpDir := t.TempDir()
	store := &SpyConfigStore{homeResult: tmpDir}
	cli := NewCLI(in, out, errOut, store, spySyncer, nil, spyAPI, spyCreds, logger)

	if err := cli.Run(context.TODO(), "any-config.yaml", "", ""); err != nil {
		t.Fatalf("Run() returned an unexpected error: %v", err)
	}

	if !spySyncer.syncCalled {
		t.Fatal("expected Sync to be called")
	}
	expectedRootDir := filepath.Join(tmpDir, "runners")
	if spySyncer.syncRootDir != expectedRootDir {
		t.Fatalf("expected root dir %s, got %s", expectedRootDir, spySyncer.syncRootDir)
	}
	if len(spySyncer.syncCfg.Runners) != 1 || spySyncer.syncCfg.Runners[0].Name != "api-runner" {
		t.Fatalf("expected runners from API to be used, got %+v", spySyncer.syncCfg.Runners)
	}
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

	tmpDir := t.TempDir()
	h := installTestHarness{
		api:    &SpyAPIClient{machineID: "test-id", machineToken: "test-token"},
		creds:  &SpyCredentialStore{},
		sm:     &SpyServiceManager{},
		syncer: &SpySyncer{},
		store:  &SpyConfigStore{homeResult: tmpDir},
	}

	h.cli = NewCLI(in, out, errOut, h.store, h.syncer, h.sm, h.api, h.creds, logger)
	return h
}

func TestCLI_Install(t *testing.T) {
	t.Run("happy path performs all steps", func(t *testing.T) {
		h := newInstallTestHarness(t)
		setupMockConfig(t, &h)

		err := h.cli.Install(context.TODO(), "test-jwt")

		if err != nil {
			t.Fatalf("Install() returned an unexpected error: %v", err)
		}
		assertInstallStepsExecuted(t, h)
	})

	t.Run("is idempotent when credentials already exist", func(t *testing.T) {
		h := newInstallTestHarness(t)
		setupMockConfig(t, &h)
		h.creds.loadID = "existing-id"
		h.creds.loadToken = "existing-token"

		err := h.cli.Install(context.TODO(), "")
		if err != nil {
			t.Fatalf("Install() returned an unexpected error: %v", err)
		}

		if !h.creds.loadCalled {
			t.Fatalf("expected credential load to be called")
		}
		if h.api.registerMachineCalled {
			t.Fatalf("expected RegisterMachine not to be called when credentials exist")
		}
		if h.creds.saveCalled {
			t.Fatalf("expected Save not to be called when credentials exist")
		}
		if !h.syncer.syncCalled {
			t.Fatalf("expected Sync to be called")
		}
		if !h.sm.installCalled {
			t.Fatalf("expected ServiceManager.Install to be called")
		}
	})

	t.Run("succeeds even if init has not been run (managed mode)", func(t *testing.T) {
		h := newInstallTestHarness(t)
		h.store.existsResult = false // Simulate config not existing
		err := h.cli.Install(context.TODO(), "test-jwt")
		if err != nil {
			t.Fatalf("Install() returned an unexpected error: %v", err)
		}
		assertInstallStepsExecuted(t, h)
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
	configPath := filepath.Join(h.store.UrsoHome(), "config.yaml")
	configContent := []byte(`runners: []`)
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
	if !h.syncer.syncCalled {
		t.Error("Sync was not called")
	}
	if !h.sm.installCalled {
		t.Error("ServiceManager.Install was not called")
	}
}

func TestCLI_Uninstall(t *testing.T) {
	t.Run("happy path calls service manager uninstall", func(t *testing.T) {
		h := newInstallTestHarness(t)

		err := h.cli.Uninstall(context.TODO())

		if err != nil {
			t.Fatalf("Uninstall() returned an unexpected error: %v", err)
		}
		if !h.sm.uninstallCalled {
			t.Error("expected ServiceManager.Uninstall to be called")
		}
	})

	t.Run("returns error if service manager is nil", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, nil, nil, nil, nil, nil, logger)

		err := cli.Uninstall(context.TODO())
		if err == nil {
			t.Fatal("expected an error but got nil")
		}
		if !errors.Is(err, ErrUnsupportedOS) {
			t.Errorf("expected ErrUnsupportedOS, got %v", err)
		}
	})
}
