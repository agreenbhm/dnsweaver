package adguard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

func TestClient_Ping(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   serverStatus
		wantErr    bool
		wantErrIs  error
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			response:   serverStatus{Version: "v0.107.55", Running: true, ProtectionEnabled: true},
			wantErr:    false,
		},
		{
			name:       "not running",
			statusCode: http.StatusOK,
			response:   serverStatus{Version: "v0.107.55", Running: false},
			wantErr:    true,
			wantErrIs:  provider.ErrProviderUnavailable,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
			wantErrIs:  provider.ErrUnauthorized,
		},
		{
			name:       "forbidden",
			statusCode: http.StatusForbidden,
			wantErr:    true,
			wantErrIs:  provider.ErrUnauthorized,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/control/status" {
					t.Errorf("unexpected path: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method: %s", r.Method)
				}

				// Verify basic auth
				user, pass, ok := r.BasicAuth()
				if !ok || user != "admin" || pass != "secret" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin", "secret",
				WithHTTPClient(server.Client()),
			)

			err := client.Ping(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Ping() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErrIs != nil && err != nil {
				if !errorIs(err, tt.wantErrIs) {
					t.Errorf("Ping() error = %v, want %v", err, tt.wantErrIs)
				}
			}
		})
	}
}

func TestClient_List(t *testing.T) {
	enabled := true
	disabled := false

	tests := []struct {
		name       string
		statusCode int
		response   []rewriteEntry
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "success with entries",
			statusCode: http.StatusOK,
			response: []rewriteEntry{
				{Domain: "server.home.local", Answer: "192.168.1.100", Enabled: &enabled},
				{Domain: "nas.home.local", Answer: "192.168.1.200", Enabled: &enabled},
				{Domain: "alias.home.local", Answer: "server.home.local", Enabled: &enabled},
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:       "empty list",
			statusCode: http.StatusOK,
			response:   []rewriteEntry{},
			wantCount:  0,
			wantErr:    false,
		},
		{
			name:       "includes disabled entries",
			statusCode: http.StatusOK,
			response: []rewriteEntry{
				{Domain: "active.local", Answer: "1.2.3.4", Enabled: &enabled},
				{Domain: "disabled.local", Answer: "5.6.7.8", Enabled: &disabled},
			},
			wantCount: 2, // Client returns all; provider filters disabled
			wantErr:   false,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/control/rewrite/list" {
					t.Errorf("unexpected path: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.response)
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin", "secret",
				WithHTTPClient(server.Client()),
			)

			entries, err := client.List(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(entries) != tt.wantCount {
				t.Errorf("List() returned %d entries, want %d", len(entries), tt.wantCount)
			}
		})
	}
}

func TestClient_Create(t *testing.T) {
	tests := []struct {
		name       string
		entry      rewriteEntry
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success",
			entry:      rewriteEntry{Domain: "new.home.local", Answer: "10.0.0.1"},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "unauthorized",
			entry:      rewriteEntry{Domain: "new.home.local", Answer: "10.0.0.1"},
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "server error",
			entry:      rewriteEntry{Domain: "new.home.local", Answer: "10.0.0.1"},
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/control/rewrite/add" {
					t.Errorf("unexpected path: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s, want POST", r.Method)
				}

				var received rewriteEntry
				if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}

				if received.Domain != tt.entry.Domain || received.Answer != tt.entry.Answer {
					t.Errorf("Create() sent %+v, want %+v", received, tt.entry)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin", "secret",
				WithHTTPClient(server.Client()),
			)

			err := client.Create(context.Background(), tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_Delete(t *testing.T) {
	tests := []struct {
		name       string
		entry      rewriteEntry
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success",
			entry:      rewriteEntry{Domain: "old.home.local", Answer: "10.0.0.1"},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "server error",
			entry:      rewriteEntry{Domain: "old.home.local", Answer: "10.0.0.1"},
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/control/rewrite/delete" {
					t.Errorf("unexpected path: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s, want POST", r.Method)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin", "secret",
				WithHTTPClient(server.Client()),
			)

			err := client.Delete(context.Background(), tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_Update(t *testing.T) {
	tests := []struct {
		name       string
		target     rewriteEntry
		update     rewriteEntry
		statusCode int
		wantErr    bool
	}{
		{
			name:       "success",
			target:     rewriteEntry{Domain: "server.local", Answer: "10.0.0.1"},
			update:     rewriteEntry{Domain: "server.local", Answer: "10.0.0.2"},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "server error",
			target:     rewriteEntry{Domain: "server.local", Answer: "10.0.0.1"},
			update:     rewriteEntry{Domain: "server.local", Answer: "10.0.0.2"},
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/control/rewrite/update" {
					t.Errorf("unexpected path: %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				if r.Method != http.MethodPut {
					t.Errorf("unexpected method: %s, want PUT", r.Method)
				}

				var received rewriteUpdate
				if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}

				if received.Target.Domain != tt.target.Domain || received.Target.Answer != tt.target.Answer {
					t.Errorf("Update() target = %+v, want %+v", received.Target, tt.target)
				}
				if received.Update.Domain != tt.update.Domain || received.Update.Answer != tt.update.Answer {
					t.Errorf("Update() update = %+v, want %+v", received.Update, tt.update)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "admin", "secret",
				WithHTTPClient(server.Client()),
			)

			err := client.Update(context.Background(), tt.target, tt.update)
			if (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			t.Error("no basic auth header sent")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if user != "testuser" || pass != "testpass" {
			t.Errorf("wrong credentials: user=%q, pass=%q", user, pass)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(serverStatus{Version: "v0.107.55", Running: true})
	}))
	defer server.Close()

	client := NewClient(server.URL, "testuser", "testpass",
		WithHTTPClient(server.Client()),
	)

	err := client.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping() with correct auth failed: %v", err)
	}
}

// errorIs checks if an error is or wraps a target error.
func errorIs(err, target error) bool {
	return errors.Is(err, target)
}
