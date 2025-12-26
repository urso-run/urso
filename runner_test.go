package urso

import (
	"log/slog"
	"testing"
)

// --- Test Doubles (Spies) ---

type SpyMachineInspector struct {
	stateToReturn            MachineState
	errToReturn              error
	GetCurrentStateWasCalled bool
	GetCurrentStateRootDir   string
	RemoveAllWasCalled       bool
}

func (s *SpyMachineInspector) GetCurrentState(rootDir string) (MachineState, error) {
	s.GetCurrentStateWasCalled = true
	s.GetCurrentStateRootDir = rootDir
	return s.stateToReturn, s.errToReturn
}

func (s *SpyMachineInspector) EnsureRootDirExists(_ string) error { return nil }
func (s *SpyMachineInspector) CreateTempDir(_, _ string) (string, error) {
	return "/tmp/fake-runner-archive", nil
}
func (s *SpyMachineInspector) RemoveAll(_ string) error {
	s.RemoveAllWasCalled = true
	return nil
}
func (s *SpyMachineInspector) MkdirAll(_ string) error { return nil }

type SpyActionsDownloader struct {
	archivePathToReturn       string
	errToReturn               error
	GetRunnerArchiveWasCalled bool
	GetRunnerArchiveDstDir    string
}

func (s *SpyActionsDownloader) GetRunnerArchive(dstDir string) (string, error) {
	s.GetRunnerArchiveWasCalled = true
	s.GetRunnerArchiveDstDir = dstDir
	return s.archivePathToReturn, s.errToReturn
}

type SpyRunnerExecutor struct {
	ExtractWasCalled          bool
	ConfigureWasCalled        bool
	InstallServiceWasCalled   bool
	StartServiceWasCalled     bool
	UninstallServiceWasCalled bool
	UnconfigureWasCalled      bool
	ConfigureDir              string
	ConfigureCfg              RunnerConfig
	ConfigureTok              string
	UninstallDir              string
	UnconfigureDir            string
	RemoveTok                 string
}

func (s *SpyRunnerExecutor) Extract(_, _ string) error {
	s.ExtractWasCalled = true
	return nil
}
func (s *SpyRunnerExecutor) Configure(dir string, cfg RunnerConfig, token string) error {
	s.ConfigureWasCalled = true
	s.ConfigureDir = dir
	s.ConfigureCfg = cfg
	s.ConfigureTok = token
	return nil
}
func (s *SpyRunnerExecutor) InstallService(_ string) error {
	s.InstallServiceWasCalled = true
	return nil
}
func (s *SpyRunnerExecutor) StartService(_ string) error {
	s.StartServiceWasCalled = true
	return nil
}
func (s *SpyRunnerExecutor) UninstallService(dir string) error {
	s.UninstallServiceWasCalled = true
	s.UninstallDir = dir
	return nil
}
func (s *SpyRunnerExecutor) Unconfigure(dir string, token string) error {
	s.UnconfigureWasCalled = true
	s.UnconfigureDir = dir
	s.RemoveTok = token
	return nil
}

// --- Test Harness ---

type testHarness struct {
	machine    *SpyMachineInspector
	downloader *SpyActionsDownloader
	executor   *SpyRunnerExecutor
	syncer     *RunnerSyncer
}

func newTestHarness() testHarness {
	machine := &SpyMachineInspector{}
	downloader := &SpyActionsDownloader{}
	executor := &SpyRunnerExecutor{}
	logger := slog.New(slog.DiscardHandler)
	syncer := NewRunnerSyncer(machine, downloader, executor, logger)
	return testHarness{machine, downloader, executor, syncer}
}

// --- Assertion Helpers ---

func assertCreatesRunner(t *testing.T, h testHarness, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Sync() returned an unexpected error: %v", err)
	}
	if !h.downloader.GetRunnerArchiveWasCalled {
		t.Error("expected GetRunnerArchive to be called, but it wasn't")
	}
	if !h.executor.ConfigureWasCalled {
		t.Error("expected Configure to be called, but it wasn't")
	}
	if h.executor.ConfigureCfg.Name != "new-runner" {
		t.Errorf("got runner name %q, want 'new-runner'", h.executor.ConfigureCfg.Name)
	}
	if !h.executor.InstallServiceWasCalled {
		t.Error("expected InstallService to be called, but it wasn't")
	}
	if !h.executor.StartServiceWasCalled {
		t.Error("expected StartService to be called, but it wasn't")
	}
	if h.executor.UnconfigureWasCalled {
		t.Error("expected Unconfigure NOT to be called, but it was")
	}
}

func assertRemovesRunner(t *testing.T, h testHarness, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Sync() returned an unexpected error: %v", err)
	}
	if h.downloader.GetRunnerArchiveWasCalled {
		t.Error("expected GetRunnerArchive NOT to be called, but it was")
	}
	if h.executor.ConfigureWasCalled {
		t.Error("expected Configure NOT to be called, but it was")
	}
	if !h.executor.UninstallServiceWasCalled {
		t.Error("expected UninstallService to be called, but it wasn't")
	}
	if !h.executor.UnconfigureWasCalled {
		t.Error("expected Unconfigure to be called, but it wasn't")
	}
	expectedRunnerDir := "/test/runners/old-runner"
	if h.executor.UnconfigureDir != expectedRunnerDir {
		t.Errorf("got removal dir %q, want %q", h.executor.UnconfigureDir, expectedRunnerDir)
	}
}

func assertDoesNothing(t *testing.T, h testHarness, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Sync() returned an unexpected error: %v", err)
	}
	if h.downloader.GetRunnerArchiveWasCalled {
		t.Error("downloader was called but should not have been")
	}
	if h.executor.ConfigureWasCalled || h.executor.UnconfigureWasCalled {
		t.Error("executor was called but should not have been")
	}
}

func assertCreatesAndRemoves(t *testing.T, h testHarness, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Sync() returned an unexpected error: %v", err)
	}
	if !h.downloader.GetRunnerArchiveWasCalled {
		t.Error("expected GetRunnerArchive to be called for new runner")
	}
	if !h.executor.ConfigureWasCalled {
		t.Error("expected Configure to be called for new runner")
	}
	if h.executor.ConfigureCfg.Name != "new-runner" {
		t.Errorf("got new runner name %q, want 'new-runner'", h.executor.ConfigureCfg.Name)
	}
	if !h.executor.UnconfigureWasCalled {
		t.Error("expected Unconfigure to be called for old runner")
	}
	expectedOldRunnerDir := "/test/runners/old-runner"
	if h.executor.UnconfigureDir != expectedOldRunnerDir {
		t.Errorf("got removal dir %q, want %q", h.executor.UnconfigureDir, expectedOldRunnerDir)
	}
}

func assertPassesCorrectTokens(t *testing.T, h testHarness, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Sync() returned an unexpected error: %v", err)
	}
	if h.executor.ConfigureTok != "REGISTER_TOKEN_123" {
		t.Errorf("got register token %q, want 'REGISTER_TOKEN_123'", h.executor.ConfigureTok)
	}
	if h.executor.RemoveTok != "REMOVE_TOKEN_456" {
		t.Errorf("got remove token %q, want 'REMOVE_TOKEN_456'", h.executor.RemoveTok)
	}
}

// --- Tests ---

func TestRunnerSyncer_Sync(t *testing.T) {
	testCases := []struct {
		name          string
		initialState  MachineState
		config        Config
		registerToken string
		removeToken   string
		assert        func(t *testing.T, h testHarness, err error)
	}{
		{
			name:         "creates a runner",
			initialState: MachineState{Runners: make(map[string]struct{})},
			config:       Config{RootDir: "/test/runners", Runners: []RunnerConfig{{Name: "new-runner", URL: "http://example.com"}}},
			assert:       assertCreatesRunner,
		},
		{
			name:         "removes a runner",
			initialState: MachineState{Runners: map[string]struct{}{"old-runner": {}}},
			config:       Config{RootDir: "/test/runners", Runners: []RunnerConfig{}},
			assert:       assertRemovesRunner,
		},
		{
			name:         "does nothing when in sync",
			initialState: MachineState{Runners: map[string]struct{}{"existing-runner": {}}},
			config:       Config{RootDir: "/test/runners", Runners: []RunnerConfig{{Name: "existing-runner"}}},
			assert:       assertDoesNothing,
		},
		{
			name:         "creates and removes in the same run",
			initialState: MachineState{Runners: map[string]struct{}{"old-runner": {}}},
			config:       Config{RootDir: "/test/runners", Runners: []RunnerConfig{{Name: "new-runner"}}},
			assert:       assertCreatesAndRemoves,
		},
		{
			name:          "passes correct tokens",
			initialState:  MachineState{Runners: map[string]struct{}{"old-runner": {}}},
			config:        Config{RootDir: "/test/runners", Runners: []RunnerConfig{{Name: "new-runner"}}},
			registerToken: "REGISTER_TOKEN_123",
			removeToken:   "REMOVE_TOKEN_456",
			assert:        assertPassesCorrectTokens,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness()
			h.machine.stateToReturn = tc.initialState

			registerToken := "default-reg-token"
			if tc.registerToken != "" {
				registerToken = tc.registerToken
			}
			removeToken := "default-rem-token"
			if tc.removeToken != "" {
				removeToken = tc.removeToken
			}

			err := h.syncer.Sync(tc.config, registerToken, removeToken)

			if tc.assert != nil {
				tc.assert(t, h, err)
			}
		})
	}
}
