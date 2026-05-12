package provider

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// identifiableMock extends mockProvider to expose a configurable Identity
// for testing per-identity precedence (issue #88).
type identifiableMock struct {
	mockProvider
	identity ProviderIdentity
}

func (m *identifiableMock) Identity() ProviderIdentity { return m.identity }

// TestIdentityOf_Fallback verifies that providers that do NOT implement
// Identifiable fall back to a type-only identity, preserving #86's
// first-match-wins behavior for legacy out-of-tree providers.
func TestIdentityOf_Fallback(t *testing.T) {
	p := &mockProvider{name: "legacy", typeName: "legacy-type"}
	got := IdentityOf(p)
	want := ProviderIdentity{Type: "legacy-type"}
	if got != want {
		t.Errorf("IdentityOf(legacy) = %+v, want %+v", got, want)
	}
}

// TestIdentityOf_Identifiable verifies that providers implementing
// Identifiable get their reported identity surfaced.
func TestIdentityOf_Identifiable(t *testing.T) {
	want := ProviderIdentity{Type: "cloudflare", Endpoint: "https://api.cloudflare.com", Zone: "example.com"}
	p := &identifiableMock{
		mockProvider: mockProvider{name: "cf", typeName: "cloudflare"},
		identity:     want,
	}
	if got := IdentityOf(p); got != want {
		t.Errorf("IdentityOf(identifiable) = %+v, want %+v", got, want)
	}
}

// TestRegistry_CreateInstance_PopulatesIdentity verifies that the registry
// computes and stores the backend identity on each ProviderInstance at
// creation time, so the reconciler doesn't have to recompute it per
// reconcile.
func TestRegistry_CreateInstance_PopulatesIdentity(t *testing.T) {
	wantIdentity := ProviderIdentity{Type: "test-type", Endpoint: "http://t", Zone: "z"}
	r := NewRegistry(testLogger())
	r.RegisterFactory("test", func(cfg FactoryConfig) (Provider, error) {
		return &identifiableMock{
			mockProvider: mockProvider{name: cfg.Name, typeName: "test-type"},
			identity:     wantIdentity,
		}, nil
	})

	if err := r.CreateInstance(ProviderInstanceConfig{
		Name:       "i1",
		TypeName:   "test",
		RecordType: RecordTypeA,
		Target:     "10.0.0.1",
		TTL:        300,
		Domains:    []string{"example.com"},
	}); err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}

	inst, ok := r.Get("i1")
	if !ok {
		t.Fatal("instance not found")
	}
	if inst.Identity != wantIdentity {
		t.Errorf("Identity = %+v, want %+v", inst.Identity, wantIdentity)
	}
}

// TestRegistry_WarnDuplicateIdentities verifies that the startup-time
// validation surfaces collisions between instances that share the same
// (Identity, RecordType) tuple. Reflects issue #88: such instances will
// be collapsed by the reconciler's per-identity precedence, so the user
// must be told once at startup.
func TestRegistry_WarnDuplicateIdentities(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := NewRegistry(logger)

	sharedIdentity := ProviderIdentity{Type: "test-type", Endpoint: "http://shared", Zone: "example.com"}
	distinctIdentity := ProviderIdentity{Type: "test-type", Endpoint: "http://other", Zone: "example.com"}

	r.RegisterFactory("shared", func(cfg FactoryConfig) (Provider, error) {
		return &identifiableMock{
			mockProvider: mockProvider{name: cfg.Name, typeName: "test-type"},
			identity:     sharedIdentity,
		}, nil
	})
	r.RegisterFactory("distinct", func(cfg FactoryConfig) (Provider, error) {
		return &identifiableMock{
			mockProvider: mockProvider{name: cfg.Name, typeName: "test-type"},
			identity:     distinctIdentity,
		}, nil
	})

	mustCreate := func(name, typ string) {
		t.Helper()
		if err := r.CreateInstance(ProviderInstanceConfig{
			Name: name, TypeName: typ,
			RecordType: RecordTypeA, Target: "10.0.0.1", TTL: 300,
			Domains: []string{"example.com"},
		}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	mustCreate("dup-1", "shared")
	mustCreate("dup-2", "shared")
	mustCreate("solo", "distinct")

	logBuf.Reset()
	r.WarnDuplicateIdentities()

	out := logBuf.String()
	if !strings.Contains(out, "multiple provider instances share the same backend identity") {
		t.Errorf("expected duplicate-identity warning, got:\n%s", out)
	}
	if !strings.Contains(out, "dup-1") || !strings.Contains(out, "dup-2") {
		t.Errorf("warning should name the colliding instances, got:\n%s", out)
	}
	if strings.Contains(out, "solo") {
		t.Errorf("non-colliding instance should not appear in warnings, got:\n%s", out)
	}
}

// TestRegistry_WarnDuplicateIdentities_DifferentRecordType verifies that
// two instances with the same Identity but different RecordType (e.g. A vs
// AAAA on the same Cloudflare zone) are NOT considered colliding — they
// own disjoint record sets.
func TestRegistry_WarnDuplicateIdentities_DifferentRecordType(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	r := NewRegistry(logger)

	identity := ProviderIdentity{Type: "test-type", Endpoint: "http://shared", Zone: "example.com"}
	r.RegisterFactory("test", func(cfg FactoryConfig) (Provider, error) {
		return &identifiableMock{
			mockProvider: mockProvider{name: cfg.Name, typeName: "test-type"},
			identity:     identity,
		}, nil
	})

	if err := r.CreateInstance(ProviderInstanceConfig{
		Name: "ipv4", TypeName: "test",
		RecordType: RecordTypeA, Target: "10.0.0.1", TTL: 300,
		Domains: []string{"example.com"},
	}); err != nil {
		t.Fatalf("create ipv4: %v", err)
	}
	if err := r.CreateInstance(ProviderInstanceConfig{
		Name: "ipv6", TypeName: "test",
		RecordType: RecordTypeAAAA, Target: "2001:db8::1", TTL: 300,
		Domains: []string{"example.com"},
	}); err != nil {
		t.Fatalf("create ipv6: %v", err)
	}

	r.WarnDuplicateIdentities()

	if strings.Contains(logBuf.String(), "multiple provider instances share the same backend identity") {
		t.Errorf("different record types should not warn, got:\n%s", logBuf.String())
	}
}
