package reconciler

import (
	"context"
	"fmt"
	"testing"

	"github.com/maxfield-allison/dnsweaver/pkg/provider"
	"github.com/maxfield-allison/dnsweaver/pkg/source"
)

// noTXTCapabilities returns capabilities for a provider that doesn't support TXT records.
func noTXTCapabilities() *provider.Capabilities {
	return &provider.Capabilities{
		SupportsOwnershipTXT: false,
		SupportsNativeUpdate: true,
		SupportedRecordTypes: []provider.RecordType{
			provider.RecordTypeA,
			provider.RecordTypeAAAA,
			provider.RecordTypeCNAME,
		},
	}
}

// --- deleteTargetMatchForProvider unit tests ---

func TestDeleteTargetMatch_DeletesMatchingRecord(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	// The record in the provider matches this instance's type+target
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1",
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, cache)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d: %+v", len(actions), actions)
	}
	if actions[0].Type != ActionDelete {
		t.Errorf("expected ActionDelete, got %s", actions[0].Type)
	}
	if actions[0].Status != StatusSuccess {
		t.Errorf("expected StatusSuccess, got %s (error: %s)", actions[0].Status, actions[0].Error)
	}
	if actions[0].Target != "10.0.0.1" {
		t.Errorf("expected target 10.0.0.1, got %s", actions[0].Target)
	}

	// Verify the record was actually deleted from the mock
	deleted := mock.GetDeleted()
	if len(deleted) != 1 {
		t.Fatalf("expected 1 deleted record, got %d", len(deleted))
	}
	if deleted[0].Target != "10.0.0.1" {
		t.Errorf("deleted wrong record: target=%s", deleted[0].Target)
	}
}

func TestDeleteTargetMatch_SkipsNonMatchingTarget(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	// Record has different target than instance
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.99.99.99", // manual record with different target
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1", // instance target is different
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.99.99.99"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, cache)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d: %+v", len(actions), actions)
	}
	if actions[0].Type != ActionSkip {
		t.Errorf("expected ActionSkip, got %s", actions[0].Type)
	}
	if actions[0].Status != StatusSkipped {
		t.Errorf("expected StatusSkipped, got %s", actions[0].Status)
	}

	// Verify nothing was deleted
	deleted := mock.GetDeleted()
	if len(deleted) != 0 {
		t.Errorf("expected no deletions, got %d: %+v", len(deleted), deleted)
	}
}

func TestDeleteTargetMatch_SkipsNonMatchingRecordType(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	// Record has same target but different type (CNAME vs A)
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeCNAME,
		Target:   "10.0.0.1", // same IP but as CNAME (unusual but tests the type check)
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeCNAME, Target: "10.0.0.1"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, cache)

	if len(actions) != 1 {
		t.Fatalf("expected 1 skip action, got %d: %+v", len(actions), actions)
	}
	if actions[0].Type != ActionSkip {
		t.Errorf("expected ActionSkip, got %s", actions[0].Type)
	}

	deleted := mock.GetDeleted()
	if len(deleted) != 0 {
		t.Errorf("expected no deletions, got %d", len(deleted))
	}
}

func TestDeleteTargetMatch_MixedRecords_OnlyDeletesMatching(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	// Multiple records for same hostname: one matches, two don't
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1", // matches instance
	})
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.99.99.99", // different target (manual)
	})
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeCNAME,
		Target:   "proxy.example.com", // different type
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1"},
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.99.99.99"},
					{Hostname: "app.home.example.com", Type: provider.RecordTypeCNAME, Target: "proxy.example.com"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, cache)

	// Should only delete the one matching record
	deleteCount := 0
	for _, a := range actions {
		if a.Type == ActionDelete && a.Status == StatusSuccess {
			deleteCount++
			if a.Target != "10.0.0.1" {
				t.Errorf("deleted wrong target: got %s, want 10.0.0.1", a.Target)
			}
		}
	}
	if deleteCount != 1 {
		t.Errorf("expected 1 deletion, got %d; actions: %+v", deleteCount, actions)
	}

	// Verify only one record deleted in mock
	deleted := mock.GetDeleted()
	if len(deleted) != 1 {
		t.Fatalf("expected 1 deleted record, got %d: %+v", len(deleted), deleted)
	}
	if deleted[0].Target != "10.0.0.1" {
		t.Errorf("deleted wrong record: target=%s", deleted[0].Target)
	}
}

func TestDeleteTargetMatch_DryRun(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")

	// Seed a record so the provider's List() returns a matching record for dry-run
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1",
	})

	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true, DryRun: true},
	}
	rec.syncAtomics()

	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, nil)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != ActionDelete {
		t.Errorf("expected ActionDelete, got %s", actions[0].Type)
	}
	if actions[0].Status != StatusSuccess {
		t.Errorf("expected StatusSuccess for dry-run, got %s", actions[0].Status)
	}

	// Verify nothing was actually deleted
	deleted := mock.GetDeleted()
	if len(deleted) != 0 {
		t.Errorf("dry-run should not delete records, got %d deletions", len(deleted))
	}
}

func TestDeleteTargetMatch_FallsBackToProviderList(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	// Records exist in the mock provider (no cache)
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1",
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	// No cache — should fall back to List()
	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, nil)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d: %+v", len(actions), actions)
	}
	if actions[0].Type != ActionDelete || actions[0].Status != StatusSuccess {
		t.Errorf("expected successful delete, got type=%s status=%s error=%s",
			actions[0].Type, actions[0].Status, actions[0].Error)
	}
}

func TestDeleteTargetMatch_ListError(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()
	mock.listErr = fmt.Errorf("connection refused")

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	// No cache — forces List() call which fails
	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, nil)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %s", actions[0].Status)
	}
	if actions[0].Error == "" {
		t.Error("expected error message, got empty string")
	}
}

func TestDeleteTargetMatch_DeleteError(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()
	mock.deleteFn = func(_ context.Context, _ provider.Record) error {
		return fmt.Errorf("delete failed: timeout")
	}

	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1",
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, cache)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %s", actions[0].Status)
	}
}

func TestDeleteTargetMatch_NoRecordsForHostname(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	// Provider has no records for this hostname
	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	// Empty cache for this hostname
	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {},
		},
		logger: logger,
	}

	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, cache)

	if len(actions) != 1 {
		t.Fatalf("expected 1 skip action, got %d", len(actions))
	}
	if actions[0].Type != ActionSkip {
		t.Errorf("expected ActionSkip, got %s", actions[0].Type)
	}
}

func TestDeleteTargetMatch_AAAA(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeAAAA,
		Target:   "2001:db8::1",
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "AAAA",
		Target:     "2001:db8::1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeAAAA, Target: "2001:db8::1"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, cache)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != ActionDelete || actions[0].Status != StatusSuccess {
		t.Errorf("expected successful delete for AAAA, got type=%s status=%s", actions[0].Type, actions[0].Status)
	}
}

func TestDeleteTargetMatch_CNAME(t *testing.T) {
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeCNAME,
		Target:   "proxy.example.com",
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "CNAME",
		Target:     "proxy.example.com",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config:    Config{OwnershipTracking: true},
	}
	rec.syncAtomics()

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeCNAME, Target: "proxy.example.com"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteTargetMatchForProvider(context.Background(), "app.home.example.com", inst, cache)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != ActionDelete || actions[0].Status != StatusSuccess {
		t.Errorf("expected successful delete for CNAME, got type=%s status=%s", actions[0].Type, actions[0].Status)
	}
}

// --- Integration: deleteOrphanForProvider routes correctly ---

func TestDeleteOrphanForProvider_RoutesToTargetMatch(t *testing.T) {
	// Verify that the decision tree in deleteOrphanForProvider correctly routes
	// to deleteTargetMatchForProvider when: managed mode + ownership tracking ON +
	// provider does NOT support TXT.
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1",
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("adguard")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config: Config{
			CleanupOrphans:    true,
			OwnershipTracking: true, // ON
		},
	}
	rec.syncAtomics()

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteOrphanForProvider(context.Background(), "app.home.example.com", inst, cache)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d: %+v", len(actions), actions)
	}
	if actions[0].Type != ActionDelete {
		t.Errorf("expected ActionDelete, got %s", actions[0].Type)
	}
	if actions[0].Status != StatusSuccess {
		t.Errorf("expected StatusSuccess, got %s (error: %s)", actions[0].Status, actions[0].Error)
	}
}

func TestDeleteOrphanForProvider_RoutesToManagedTXT(t *testing.T) {
	// Verify that a provider WITH TXT support still uses the TXT-based path.
	logger := quietLogger()
	mock := newTestMockProvider("technitium")
	// Default capabilities include SupportsOwnershipTXT: true

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "technitium",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("technitium")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config: Config{
			CleanupOrphans:    true,
			OwnershipTracking: true,
			InstanceID:        "test-instance",
		},
	}
	rec.syncAtomics()

	// No ownership TXT record exists — managed mode should SKIP (not delete)
	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"technitium": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteOrphanForProvider(context.Background(), "app.home.example.com", inst, cache)

	// Without ownership record, managed TXT path should SKIP
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d: %+v", len(actions), actions)
	}
	if actions[0].Type != ActionSkip {
		t.Errorf("expected ActionSkip (no TXT record), got %s", actions[0].Type)
	}
}

func TestDeleteOrphanForProvider_NoTXT_NoMatchingTarget_Skips(t *testing.T) {
	// Provider without TXT, managed mode, record target doesn't match instance → skip
	logger := quietLogger()
	mock := newTestMockProvider("pihole-file")
	mock.capabilities = noTXTCapabilities()

	mock.AddRecord(provider.Record{
		Hostname: "manual.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.99.99.99", // manual record
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "pihole-file",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	inst, _ := reg.Get("pihole-file")
	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config: Config{
			CleanupOrphans:    true,
			OwnershipTracking: true,
		},
	}
	rec.syncAtomics()

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"pihole-file": {
				"manual.home.example.com": {
					{Hostname: "manual.home.example.com", Type: provider.RecordTypeA, Target: "10.99.99.99"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.deleteOrphanForProvider(context.Background(), "manual.home.example.com", inst, cache)

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != ActionSkip {
		t.Errorf("expected ActionSkip for non-matching target, got %s", actions[0].Type)
	}
}

// --- Integration: full cleanupOrphans flow ---

func TestCleanupOrphans_TargetMatch_FullFlow(t *testing.T) {
	// End-to-end: a container stops, its hostname becomes orphaned,
	// and the AdGuard provider (no TXT) cleans it up via target matching.
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1",
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config: Config{
			CleanupOrphans:    true,
			OwnershipTracking: true,
		},
		knownHostnames: map[string]struct{}{
			"app.home.example.com": {},
		},
		hostnameProviders: map[string][]string{
			"app.home.example.com": {"adguard"},
		},
	}
	rec.syncAtomics()

	// Current state: container is gone, hostname no longer present
	currentHostnames := map[string]*source.Hostname{}

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.cleanupOrphans(context.Background(), currentHostnames, cache)

	var deleteActions []Action
	for _, a := range actions {
		if a.Type == ActionDelete && a.Status == StatusSuccess {
			deleteActions = append(deleteActions, a)
		}
	}

	if len(deleteActions) != 1 {
		t.Fatalf("expected 1 successful delete, got %d; all actions: %+v", len(deleteActions), actions)
	}
	if deleteActions[0].Hostname != "app.home.example.com" {
		t.Errorf("expected hostname app.home.example.com, got %s", deleteActions[0].Hostname)
	}
	if deleteActions[0].Target != "10.0.0.1" {
		t.Errorf("expected target 10.0.0.1, got %s", deleteActions[0].Target)
	}
}

func TestCleanupOrphans_TargetMatch_PreservesManualRecords(t *testing.T) {
	// End-to-end: a container stops, and both its record and a manual record exist.
	// Only the instance's record should be deleted; the manual one should be preserved.
	logger := quietLogger()
	mock := newTestMockProvider("adguard")
	mock.capabilities = noTXTCapabilities()

	// Two records for the same hostname — one from dnsweaver, one manual
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1", // dnsweaver's target
	})
	mock.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.50.50.50", // manual
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		return mock, nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name:       "adguard",
		TypeName:   "mock",
		Domains:    []string{"*.home.example.com"},
		RecordType: "A",
		Target:     "10.0.0.1",
		Mode:       "managed",
		TTL:        300,
	})

	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config: Config{
			CleanupOrphans:    true,
			OwnershipTracking: true,
		},
		knownHostnames: map[string]struct{}{
			"app.home.example.com": {},
		},
		hostnameProviders: map[string][]string{
			"app.home.example.com": {"adguard"},
		},
	}
	rec.syncAtomics()

	currentHostnames := map[string]*source.Hostname{}

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1"},
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.50.50.50"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.cleanupOrphans(context.Background(), currentHostnames, cache)

	var deleted []Action
	for _, a := range actions {
		if a.Type == ActionDelete && a.Status == StatusSuccess {
			deleted = append(deleted, a)
		}
	}

	if len(deleted) != 1 {
		t.Fatalf("expected exactly 1 deletion, got %d; all actions: %+v", len(deleted), actions)
	}
	if deleted[0].Target != "10.0.0.1" {
		t.Errorf("expected deleted target 10.0.0.1, got %s", deleted[0].Target)
	}

	// Verify: the manual record with target 10.50.50.50 was NOT deleted
	for _, d := range mock.GetDeleted() {
		if d.Target == "10.50.50.50" {
			t.Errorf("manual record (target 10.50.50.50) was incorrectly deleted")
		}
	}
}

func TestCleanupOrphans_MultiInstance_IndependentTargets(t *testing.T) {
	// Two dnsweaver instances pointing to different targets on the same provider.
	// Instance A's orphan should only delete A's record, not B's.
	logger := quietLogger()
	mockA := newTestMockProvider("adguard-a")
	mockA.capabilities = noTXTCapabilities()
	mockB := newTestMockProvider("adguard-b")
	mockB.capabilities = noTXTCapabilities()

	mockA.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.1", // instance A's target
	})
	mockB.AddRecord(provider.Record{
		Hostname: "app.home.example.com",
		Type:     provider.RecordTypeA,
		Target:   "10.0.0.2", // instance B's target
	})

	reg := provider.NewRegistry(logger)
	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		switch cfg.Name {
		case "adguard-a":
			return mockA, nil
		case "adguard-b":
			return mockB, nil
		}
		return newTestMockProvider(cfg.Name), nil
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name: "adguard-a", TypeName: "mock",
		Domains: []string{"*.home.example.com"}, RecordType: "A", Target: "10.0.0.1",
		Mode: "managed", TTL: 300,
	})
	_ = reg.CreateInstance(provider.ProviderInstanceConfig{
		Name: "adguard-b", TypeName: "mock",
		Domains: []string{"*.home.example.com"}, RecordType: "A", Target: "10.0.0.2",
		Mode: "managed", TTL: 300,
	})

	rec := &Reconciler{
		providers: reg,
		logger:    logger,
		config: Config{
			CleanupOrphans:    true,
			OwnershipTracking: true,
		},
		knownHostnames: map[string]struct{}{
			"app.home.example.com": {},
		},
		// Only instance A had this hostname mapped
		hostnameProviders: map[string][]string{
			"app.home.example.com": {"adguard-a"},
		},
	}
	rec.syncAtomics()

	currentHostnames := map[string]*source.Hostname{}

	cache := &recordCache{
		records: map[string]map[string][]provider.Record{
			"adguard-a": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1"},
				},
			},
			"adguard-b": {
				"app.home.example.com": {
					{Hostname: "app.home.example.com", Type: provider.RecordTypeA, Target: "10.0.0.2"},
				},
			},
		},
		logger: logger,
	}

	actions := rec.cleanupOrphans(context.Background(), currentHostnames, cache)

	// Should only delete from adguard-a, not adguard-b
	var deleted []Action
	for _, a := range actions {
		if a.Type == ActionDelete && a.Status == StatusSuccess {
			deleted = append(deleted, a)
		}
	}

	if len(deleted) != 1 {
		t.Fatalf("expected 1 deletion (from adguard-a only), got %d; all actions: %+v", len(deleted), actions)
	}
	if deleted[0].Provider != "adguard-a" {
		t.Errorf("expected deletion from adguard-a, got %s", deleted[0].Provider)
	}
	if deleted[0].Target != "10.0.0.1" {
		t.Errorf("expected target 10.0.0.1, got %s", deleted[0].Target)
	}

	// Verify instance B's record was untouched
	if len(mockB.GetDeleted()) != 0 {
		t.Errorf("adguard-b should have no deletions, got %d", len(mockB.GetDeleted()))
	}
}
