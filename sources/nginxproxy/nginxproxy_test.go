package nginxproxy

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/maxfield-allison/dnsweaver/pkg/source"
	"github.com/maxfield-allison/dnsweaver/pkg/workload"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNginxProxy_Name(t *testing.T) {
	if got := New().Name(); got != "nginx-proxy" {
		t.Errorf("Name() = %q, want %q", got, "nginx-proxy")
	}
}

func TestNginxProxy_ImplementsSource(t *testing.T) {
	var _ source.Source = (*NginxProxy)(nil)
}

func TestNginxProxy_SupportsDiscovery_False(t *testing.T) {
	if New().SupportsDiscovery() {
		t.Error("SupportsDiscovery() = true, want false")
	}
}

func TestNginxProxy_Discover_NoOp(t *testing.T) {
	hs, err := New().Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if hs != nil {
		t.Errorf("Discover() = %v, want nil", hs)
	}
}

func TestNginxProxy_Extract_Cases(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   []string
	}{
		{
			name:   "nil labels",
			labels: nil,
			want:   nil,
		},
		{
			name:   "no recognized labels",
			labels: map[string]string{"com.docker.compose.project": "x"},
			want:   nil,
		},
		{
			name: "VIRTUAL_HOST single hostname",
			labels: map[string]string{
				"VIRTUAL_HOST": "app.example.com",
			},
			want: []string{"app.example.com"},
		},
		{
			name: "VIRTUAL_HOST comma separated",
			labels: map[string]string{
				"VIRTUAL_HOST": "app.example.com,www.example.com",
			},
			want: []string{"app.example.com", "www.example.com"},
		},
		{
			name: "VIRTUAL_HOST with whitespace around comma",
			labels: map[string]string{
				"VIRTUAL_HOST": "app.example.com, www.example.com",
			},
			want: []string{"app.example.com", "www.example.com"},
		},
		{
			name: "canonical label",
			labels: map[string]string{
				"com.nginx-proxy.virtual_host": "app.example.com",
			},
			want: []string{"app.example.com"},
		},
		{
			name: "both labels — dedupe",
			labels: map[string]string{
				"VIRTUAL_HOST":                 "app.example.com",
				"com.nginx-proxy.virtual_host": "app.example.com",
			},
			want: []string{"app.example.com"},
		},
		{
			name: "both labels — distinct values merged",
			labels: map[string]string{
				"VIRTUAL_HOST":                 "app.example.com",
				"com.nginx-proxy.virtual_host": "api.example.com",
			},
			// Sorted by key: "VIRTUAL_HOST" < "com.nginx-proxy.virtual_host"
			want: []string{"app.example.com", "api.example.com"},
		},
		{
			name: "mixed case normalized",
			labels: map[string]string{
				"VIRTUAL_HOST": "APP.Example.COM",
			},
			want: []string{"app.example.com"},
		},
		{
			name: "empty value",
			labels: map[string]string{
				"VIRTUAL_HOST": "",
			},
			want: nil,
		},
		{
			name: "trailing comma",
			labels: map[string]string{
				"VIRTUAL_HOST": "app.example.com,,",
			},
			want: []string{"app.example.com"},
		},
	}

	ctx := context.Background()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := New(WithLogger(testLogger()))
			w := workload.Workload{Labels: tc.labels, Platform: workload.PlatformDocker, Kind: workload.KindContainer}

			hostnames, err := n.Extract(ctx, w)
			if err != nil {
				t.Fatalf("Extract() error = %v", err)
			}

			got := source.Hostnames(hostnames).Names()
			if !sameStringSlice(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}

			for _, h := range hostnames {
				if h.Source != "nginx-proxy" {
					t.Errorf("hostname %q has Source = %q, want nginx-proxy", h.Name, h.Source)
				}
				if h.Router == "" {
					t.Errorf("hostname %q has empty Router", h.Name)
				}
			}
		})
	}
}

func TestNginxProxy_RegistryIntegration(t *testing.T) {
	registry := source.NewRegistry(testLogger())
	if err := registry.Register(New(WithLogger(testLogger()))); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	w := workload.Workload{
		Labels:   map[string]string{"VIRTUAL_HOST": "app.example.com"},
		Platform: workload.PlatformDocker,
		Kind:     workload.KindContainer,
	}

	got := registry.ExtractAll(context.Background(), w)
	if len(got) != 1 || got[0].Name != "app.example.com" {
		t.Fatalf("ExtractAll = %v, want [app.example.com]", got)
	}
	if got[0].Source != "nginx-proxy" {
		t.Errorf("Source = %q, want nginx-proxy", got[0].Source)
	}
}

func sameStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
