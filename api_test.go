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
				w.WriteHeader(http.StatusCreated)
				if err := json.NewEncoder(w).Encode(registerMachineResponse{ID: "test-id", Token: "test-token"}); err != nil {
					t.Fatalf("failed to write response: %v", err)
				}
			},
			jwt:           "test-jwt",
			expectError:   false,
			expectedID:    "test-id",
			expectedToken: "test-token",
		},
		{
			name: "returns an error on non-201 status code",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			jwt:         "test-jwt",
			expectError: true,
		},
		{
			name: "returns an error on invalid JSON response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
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
				w.WriteHeader(http.StatusCreated)
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

			id, token, err := client.RegisterMachine(context.Background(), tc.jwt)

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
