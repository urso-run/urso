package urso

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// SpyConfigStore is a test double for the ConfigStore interface.
// It allows us to control its behavior and inspect the calls made to it.
type SpyConfigStore struct {
	// Stubbing properties: control the behavior of the spy.
	existsResult bool
	pathResult   string

	// Spying properties: record what happened.
	writeWasCalled bool
}

func (s *SpyConfigStore) Exists() bool {
	return s.existsResult
}

func (s *SpyConfigStore) Write(_ []byte) error {
	s.writeWasCalled = true
	return nil
}

func (s *SpyConfigStore) Path() string {
	return s.pathResult
}

func TestCLI_Init(t *testing.T) {
	t.Run("creates config file if it does not exist", func(t *testing.T) {
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		errOut := &bytes.Buffer{}
		store := &SpyConfigStore{
			existsResult: false,
			pathResult:   "/test/config.yaml",
		}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, store, nil, logger, "", "", "")

		err := cli.Init([]string{})

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
		in := strings.NewReader("n\n")
		out := &bytes.Buffer{}
		errOut := &bytes.Buffer{}
		store := &SpyConfigStore{
			existsResult: true,
			pathResult:   "/test/config.yaml",
		}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, store, nil, logger, "", "", "")

		err := cli.Init([]string{})

		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}
		if store.writeWasCalled {
			t.Error("expected Write() not to be called, but it was")
		}
		expectedPrompt := "Overwrite? (y/N)"
		if !strings.Contains(errOut.String(), expectedPrompt) {
			t.Errorf("expected output to contain prompt %q, but got %q", errOut.String(), expectedPrompt)
		}
		expectedAbort := "Aborted."
		if !strings.Contains(errOut.String(), expectedAbort) {
			t.Errorf("expected output to contain abort message %q, but got %q", errOut.String(), expectedAbort)
		}
	})

	t.Run("overwrites if config file exists and user says yes", func(t *testing.T) {
		in := strings.NewReader("y\n")
		out := &bytes.Buffer{}
		errOut := &bytes.Buffer{}
		store := &SpyConfigStore{
			existsResult: true,
			pathResult:   "/test/config.yaml",
		}
		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, errOut, store, nil, logger, "", "", "")

		err := cli.Init([]string{})

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
