package reconciler

import (
	"context"
	"testing"

	"github.com/maxfield-allison/dnsweaver/pkg/provider"
	"github.com/maxfield-allison/dnsweaver/pkg/source"
)

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		value string
		want  int // expected length
	}{
		{
			name:  "append to nil",
			slice: nil,
			value: "a",
			want:  1,
		},
		{
			name:  "append new value",
			slice: []string{"a"},
			value: "b",
			want:  2,
		},
		{
			name:  "skip duplicate",
			slice: []string{"a", "b"},
			value: "a",
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUnique(tt.slice, tt.value)
			if len(got) != tt.want {
				t.Errorf("appendUnique() len = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestGetOrphanProviders_UsesStoredMapping(t *testing.T) {
	// When hostnameProviders has a mapping for the hostname, getOrphanProviders
	// should return those providers instead of using domain matching.
	logger := quietLogger()

	mockInternalDNS := newTestMockProvider("internal-dns")
	mockCloudflare := newTestMockProvider("cloudflare")

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		switch cfg.Name {
		case "internal-dns":
			return mockInternalDNS, nil
		case "cloudflare":
			return mockCloudflare, nil
		}
		return newTestMockProvider(cfg.Name), nil
	})

	// Register providers with patterns:
	// internal-dns matches *.local.example.com
	// cloudflare matches *.example.com
	err := reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "internal-dns",
		TypeName:   "mock",
		Domains:    []string{"*.local.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		TTL:        300,
	})
	if err != nil {
		t.Fatalf("failed to create internal-dns instance: %v", err)
	}

	err = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "cloudflare",
		TypeName:   "mock",
		Domains:    []string{"*.example.com"},
		RecordType: "A",
		Target:     "203.0.113.1",
		TTL:        300,
	})
	if err != nil {
		t.Fatalf("failed to create cloudflare instance: %v", err)
	}

	rec := &Reconciler{
		providers:      reg,
		logger:         logger,
		knownHostnames: make(map[string]struct{}),
		// Simulate previous reconciliation that routed app.local.example.com to internal-dns
		hostnameProviders: map[string][]string{
			"app.local.example.com": {"internal-dns"},
		},
	}

	// getOrphanProviders should use the stored mapping
	providers := rec.getOrphanProviders("app.local.example.com")
	if len(providers) != 1 {
		t.Fatalf("getOrphanProviders() returned %d providers, want 1", len(providers))
	}
	if providers[0].Name() != "internal-dns" {
		t.Errorf("getOrphanProviders() returned %q, want %q", providers[0].Name(), "internal-dns")
	}
}

func TestGetOrphanProviders_FallsBackToMatching(t *testing.T) {
	// When hostnameProviders has no mapping, getOrphanProviders should fall
	// back to domain-based matching.
	logger := quietLogger()

	mockProvider := newTestMockProvider("internal-dns")
	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(_ provider.FactoryConfig) (provider.Provider, error) {
		return mockProvider, nil
	})

	err := reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "internal-dns",
		TypeName:   "mock",
		Domains:    []string{"*.local.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		TTL:        300,
	})
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}

	rec := &Reconciler{
		providers:         reg,
		logger:            logger,
		knownHostnames:    make(map[string]struct{}),
		hostnameProviders: nil, // no mapping — simulates first run after recovery
	}

	providers := rec.getOrphanProviders("app.local.example.com")
	if len(providers) != 1 {
		t.Fatalf("getOrphanProviders() returned %d providers, want 1", len(providers))
	}
	if providers[0].Name() != "internal-dns" {
		t.Errorf("getOrphanProviders() returned %q, want %q", providers[0].Name(), "internal-dns")
	}
}

func TestGetOrphanProviders_RemovedProvider(t *testing.T) {
	// When the stored mapping references a provider that no longer exists,
	// that provider should be silently skipped.
	logger := quietLogger()

	reg := provider.NewRegistry(logger)

	rec := &Reconciler{
		providers:      reg,
		logger:         logger,
		knownHostnames: make(map[string]struct{}),
		hostnameProviders: map[string][]string{
			"app.old.example.com": {"removed-provider"},
		},
	}

	providers := rec.getOrphanProviders("app.old.example.com")
	if len(providers) != 0 {
		t.Errorf("getOrphanProviders() returned %d providers, want 0 (removed provider)", len(providers))
	}
}

func TestCleanupOrphans_UsesProviderMapping(t *testing.T) {
	// Full integration test: when a hostname moves between providers,
	// the old provider's record should be cleaned up using the stored mapping.
	logger := quietLogger()

	mockInternalDNS := newTestMockProvider("internal-dns")
	mockCloudflare := newTestMockProvider("cloudflare")

	// Add the orphan record to internal-dns
	mockInternalDNS.AddRecord(provider.Record{
		Hostname: "app.local.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1",
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		switch cfg.Name {
		case "internal-dns":
			return mockInternalDNS, nil
		case "cloudflare":
			return mockCloudflare, nil
		}
		return newTestMockProvider(cfg.Name), nil
	})

	// internal-dns matches *.local.example.com
	err := reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "internal-dns",
		TypeName:   "mock",
		Domains:    []string{"*.local.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "authoritative", // Skip ownership check for simplicity
		TTL:        300,
	})
	if err != nil {
		t.Fatalf("create internal-dns: %v", err)
	}

	// cloudflare matches *.example.com
	err = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "cloudflare",
		TypeName:   "mock",
		Domains:    []string{"*.example.com"},
		RecordType: "A",
		Target:     "203.0.113.1",
		TTL:        300,
	})
	if err != nil {
		t.Fatalf("create cloudflare: %v", err)
	}

	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config: Config{
			CleanupOrphans:    true,
			OwnershipTracking: false, // Don't require ownership for this test
		},
		// Previous state: app.local.example.com was routed to internal-dns
		knownHostnames: map[string]struct{}{
			"app.local.example.com": {},
		},
		hostnameProviders: map[string][]string{
			"app.local.example.com": {"internal-dns"},
		},
	}
	rec.syncAtomics()

	// Current state: hostname changed to app.example.com (different hostname)
	currentHostnames := map[string]*source.Hostname{
		"app.example.com": {Name: "app.example.com", Source: "traefik"},
	}

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"internal-dns": {
				"app.local.example.com": {
					{Hostname: "app.local.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.cleanupOrphans(context.Background(), currentHostnames, cache)

	// Should have attempted to delete from internal-dns using stored mapping
	var deleteActions []Action
	for _, a := range actions {
		if a.Type == ActionDelete {
			deleteActions = append(deleteActions, a)
		}
	}

	if len(deleteActions) == 0 {
		t.Fatal("cleanupOrphans() produced no delete actions for orphaned hostname")
	}

	foundInternalDNS := false
	for _, a := range deleteActions {
		if a.Provider == "internal-dns" && a.Hostname == "app.local.example.com" {
			foundInternalDNS = true
		}
	}
	if !foundInternalDNS {
		t.Errorf("cleanupOrphans() did not attempt to delete from internal-dns (stored provider mapping); actions: %v", deleteActions)
	}
}
