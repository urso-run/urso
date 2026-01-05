package urso

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardAPIClient_RegisterMachine(t *testing.T) {
	testCases := []struct {
		name          string
		handler       http.HandlerFunc
		jwt           string
		hostname      string
		expectError   bool
		expectedID    string
		expectedToken string
	}{
		{
			name: "successfully registers a machine",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				authHeader := r.Header.Get("Authorization")
				if authHeader != "Bearer test-jwt" {
					t.Errorf("incorrect auth header: %s", authHeader)
				}

				hostnameHeader := r.Header.Get("Urso-Hostname")
				if hostnameHeader != "test-hostname" {
					t.Errorf("expected hostname %q, got %q", "test-hostname", hostnameHeader)
				}

				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(registerMachineResponse{ID: "test-id", Token: "test-token"}); err != nil {
					t.Fatalf("failed to write response: %v", err)
				}
			},
			jwt:           "test-jwt",
			hostname:      "test-hostname",
			expectError:   false,
			expectedID:    "test-id",
			expectedToken: "test-token",
		},
		{
			name: "returns an error on non-200 status code",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			jwt:         "test-jwt",
			expectError: true,
		},
		{
			name: "returns an error on invalid JSON response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(`{"id": "test-id", "token":`)); err != nil {
					t.Fatalf("failed to write malformed response: %v", err)
				}
			},
			jwt:         "test-jwt",
			expectError: true,
		},
		{
			name: "returns an error on response with missing fields",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte(`{"id": "test-id"}`)); err != nil {
					t.Fatalf("failed to write incomplete response: %v", err)
				}
			},
			jwt:         "test-jwt",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(tc.handler)
			defer server.Close()

			logger := slog.New(slog.DiscardHandler)
			client := &DashboardAPIClient{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
				Logger:     logger,
			}

			id, token, err := client.RegisterMachine(context.Background(), tc.jwt, tc.hostname)

			assertError(t, tc.expectError, err)

			if !tc.expectError {
				assertIDAndToken(t, id, tc.expectedID, token, tc.expectedToken)
			}
		})
	}
}

func assertError(t *testing.T, expectError bool, err error) {
	t.Helper()
	if expectError && err == nil {
		t.Fatal("expected an error but got nil")
	}
	if !expectError && err != nil {
		t.Fatalf("did not expect an error but got one: %v", err)
	}
}

func assertIDAndToken(t *testing.T, id, expectedID, token, expectedToken string) {
	t.Helper()
	if id != expectedID {
		t.Errorf("got id %q, want %q", id, expectedID)
	}
	if token != expectedToken {
		t.Errorf("got token %q, want %q", token, expectedToken)
	}
}

func TestDashboardAPIClient_GetRunnerConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/machine/test-id" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("incorrect auth header: %s", authHeader)
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(apiConfigResponse{
			Runners:   []RunnerConfig{{Name: "api-runner"}},
			DeletedAt: "2023-10-27T10:00:00Z",
		}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	client := &DashboardAPIClient{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Logger:     logger,
	}

	config, deletedAt, err := client.GetRunnerConfig(context.Background(), "test-hostname", "test-id", "test-token")

	if err != nil {
		t.Fatalf("GetRunnerConfig returned an error: %v", err)
	}
	if len(config) != 1 || config[0].Name != "api-runner" {
		t.Errorf("unexpected config returned: %+v", config)
	}
	if deletedAt != "2023-10-27T10:00:00Z" {
		t.Errorf("unexpected deletedAt returned: %q", deletedAt)
	}
}

func TestDashboardAPIClient_DeleteMachine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/api/machine/test-id" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("incorrect auth header: %s", authHeader)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	client := &DashboardAPIClient{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Logger:     logger,
	}

	err := client.DeleteMachine(context.Background(), "test-hostname", "test-id", "test-token")
	if err != nil {
		t.Fatalf("DeleteMachine returned an error: %v", err)
	}
}

func TestDashboardAPIClient_GetTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenValue string
		switch r.URL.Path {
		case "/api/machine/test-id/registration-token":
			tokenValue = "gh-reg-token"
		case "/api/machine/test-id/remove-token":
			tokenValue = "gh-rem-token"
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(tokenResponse{Token: tokenValue}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	client := &DashboardAPIClient{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Logger:     logger,
	}

	t.Run("gets register token", func(t *testing.T) {
		token, err := client.GetRegisterToken(context.Background(), "test-hostname", "test-id", "test-token", []string{"runner-1", "runner-2"})
		if err != nil {
			t.Fatalf("GetRegisterToken returned an error: %v", err)
		}
		if token != "gh-reg-token" {
			t.Errorf("got token %q, want 'gh-reg-token'", token)
		}
	})

	t.Run("gets remove token", func(t *testing.T) {
		token, err := client.GetRemoveToken(context.Background(), "test-hostname", "test-id", "test-token", []string{"runner-1", "runner-2"})
		if err != nil {
			t.Fatalf("GetRemoveToken returned an error: %v", err)
		}
		if token != "gh-rem-token" {
			t.Errorf("got token %q, want 'gh-rem-token'", token)
		}
	})
}
