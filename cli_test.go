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
	syncUrsoHome      string
	syncCfg           Config
	syncRegisterToken string
	syncRemoveToken   string
}

func (s *SpySyncer) Sync(_ context.Context, ursoHome string, cfg Config, registerProvider, removeProvider func() (string, error)) error {
	s.syncCalled = true
	s.syncUrsoHome = ursoHome
	s.syncCfg = cfg
	if registerProvider != nil {
		s.syncRegisterToken, _ = registerProvider()
	}
	if removeProvider != nil {
		s.syncRemoveToken, _ = removeProvider()
	}
	return nil
}

type SpyServiceManager struct {
	installCalled   bool
	uninstallCalled bool
	installCfg      ServiceConfig
	uninstallName   string
}

func (s *SpyServiceManager) Install(_ context.Context, cfg ServiceConfig) error {
	s.installCalled = true
	s.installCfg = cfg
	return nil
}

func (s *SpyServiceManager) Uninstall(_ context.Context, name string) error {
	s.uninstallCalled = true
	s.uninstallName = name
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

type SpyVectorManager struct {
	installCalled      bool
	uninstallCalled    bool
	updateConfigCalled bool
	machineID          string
	machineToken       string
	runners            []RunnerConfig
}

func (s *SpyVectorManager) Install(_ context.Context) error {
	s.installCalled = true
	return nil
}

func (s *SpyVectorManager) Uninstall(_ context.Context) error {
	s.uninstallCalled = true
	return nil
}

func (s *SpyVectorManager) UpdateConfig(machineID, machineToken string, runners []RunnerConfig) error {
	s.updateConfigCalled = true
	s.machineID = machineID
	s.machineToken = machineToken
	s.runners = runners
	return nil
}

// --- Tests ---

func TestCLI_Init(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)

	t.Run("creates config file if it does not exist", func(t *testing.T) {
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		store := &SpyConfigStore{existsResult: false, pathResult: "/test/config.yaml"}
		cli := NewCLI(in, out, errOut, store, nil, nil, nil, nil, nil, logger)
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
		cli := NewCLI(in, out, errOut, store, nil, nil, nil, nil, nil, logger)
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
		cli := NewCLI(in, out, errOut, store, nil, nil, nil, nil, nil, logger)
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
	cli := NewCLI(in, out, errOut, store, spySyncer, nil, nil, nil, nil, logger)

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
}

func TestCLI_Run_ManagedUpdatesVector(t *testing.T) {
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	spySyncer := &SpySyncer{}
	logger := slog.New(slog.DiscardHandler)
	spyAPI := &SpyAPIClient{
		machineID:     "mid",
		machineToken:  "mtok",
		runnerConfigs: []RunnerConfig{{Name: "api-runner", URL: "https://github.com/org"}},
	}
	spyCreds := &SpyCredentialStore{
		loadID:    "mid",
		loadToken: "mtok",
	}
	spyVector := &SpyVectorManager{}
	tmpDir := t.TempDir()
	store := &SpyConfigStore{homeResult: tmpDir}
	cli := NewCLI(in, out, errOut, store, spySyncer, nil, spyAPI, spyCreds, spyVector, logger)

	if err := cli.Run(context.TODO(), "any.yaml", "", ""); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if !spyVector.updateConfigCalled {
		t.Error("expected Vector.UpdateConfig to be called in managed mode")
	}
	if spyVector.machineID != "mid" {
		t.Errorf("expected machineID 'mid', got %q", spyVector.machineID)
	}
	if len(spyVector.runners) != 1 || spyVector.runners[0].Name != "api-runner" {
		t.Errorf("expected runner 'api-runner' passed to vector, got %+v", spyVector.runners)
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
	vm     *SpyVectorManager
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
		vm:     &SpyVectorManager{},
	}

	h.cli = NewCLI(in, out, errOut, h.store, h.syncer, h.sm, h.api, h.creds, h.vm, logger)
	return h
}

func TestCLI_Install(t *testing.T) {
	t.Run("performs vector installation", func(t *testing.T) {
		h := newInstallTestHarness(t)
		setupMockConfig(t, &h)

		err := h.cli.Install(context.TODO(), "test-jwt")
		if err != nil {
			t.Fatalf("Install() failed: %v", err)
		}

		if !h.vm.installCalled {
			t.Error("expected Vector.Install to be called during installation")
		}

		if h.syncer.syncCalled {
			t.Error("expected Sync not to be called during installation, letting the service handle it")
		}

		if h.vm.updateConfigCalled {
			t.Error("expected Vector.UpdateConfig not to be called during installation")
		}

		if h.api.getRunnerConfigCalled {
			t.Error("expected GetRunnerConfig not to be called during installation")
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

func TestCLI_Uninstall(t *testing.T) {
	t.Run("uninstalls vector service", func(t *testing.T) {
		h := newInstallTestHarness(t)
		h.creds.loadID = "mid"
		h.creds.loadToken = "mtok"

		err := h.cli.Uninstall(context.TODO())
		if err != nil {
			t.Fatalf("Uninstall() failed: %v", err)
		}

		if !h.vm.uninstallCalled {
			t.Error("expected Vector.Uninstall to be called during uninstallation")
		}
	})
}
