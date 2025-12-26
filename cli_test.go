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
		store := &SpyConfigStore{
			existsResult: false, // Tell the spy the file doesn't exist.
			pathResult:   "/test/config.yaml",
		}

		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, store, logger)
		err := cli.Init()

		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}

		if !store.writeWasCalled {
			t.Error("expected Write() to be called, but it wasn't")
		}

		// Check that a success message was printed.
		expectedOutput := "config.yaml created successfully"
		if !strings.Contains(out.String(), expectedOutput) {
			t.Errorf("expected output to contain %q, but got %q", expectedOutput, out.String())
		}
	})

	t.Run("aborts if config file exists and user says no", func(t *testing.T) {
		// We simulate the user typing "n" followed by a newline.
		in := strings.NewReader("n\n")
		out := &bytes.Buffer{}
		store := &SpyConfigStore{
			existsResult: true, // Tell the spy the file *does* exist.
			pathResult:   "/test/config.yaml",
		}

		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, store, logger)
		err := cli.Init()

		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}

		if store.writeWasCalled {
			t.Error("expected Write() not to be called, but it was")
		}

		// Check that the confirmation prompt and abort message were printed.
		expectedPrompt := "Overwrite? (y/N)"
		if !strings.Contains(out.String(), expectedPrompt) {
			t.Errorf("expected output to contain prompt %q, but got %q", expectedPrompt, out.String())
		}
		expectedAbort := "Aborted."
		if !strings.Contains(out.String(), expectedAbort) {
			t.Errorf("expected output to contain abort message %q, but got %q", expectedAbort, out.String())
		}
	})

	t.Run("overwrites if config file exists and user says yes", func(t *testing.T) {
		// We simulate the user typing "y".
		in := strings.NewReader("y\n")
		out := &bytes.Buffer{}
		store := &SpyConfigStore{
			existsResult: true, // Tell the spy the file *does* exist.
			pathResult:   "/test/config.yaml",
		}

		logger := slog.New(slog.DiscardHandler)
		cli := NewCLI(in, out, store, logger)
		err := cli.Init()

		if err != nil {
			t.Fatalf("Init() returned an unexpected error: %v", err)
		}

		if !store.writeWasCalled {
			t.Error("expected Write() to be called, but it wasn't")
		}

		// Check that a success message was printed.
		expectedOutput := "config.yaml created successfully"
		if !strings.Contains(out.String(), expectedOutput) {
			t.Errorf("expected output to contain %q, but got %q", expectedOutput, out.String())
		}
	})
}
