package testutil

import (
	"context"
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// ProviderFactory is a function that creates a Provider instance for conformance testing.
// It receives the test server URL and should return a configured provider.
type ProviderFactory func(t *testing.T, serverURL string) provider.Provider

// RunProviderConformance runs a standard set of behavioral tests against any Provider
// implementation. It validates the interface contract: Name, Type, Ping, Capabilities,
// List, Create, Delete.
//
// The factory function receives a mock server URL and should return a configured provider.
// For providers that don't use HTTP (e.g., dnsmasq), pass an empty serverURL.
//
// Tests verify behavioral contracts, not implementation details.
func RunProviderConformance(t *testing.T, name string, factory ProviderFactory) {
	t.Helper()
	t.Run("Conformance/"+name, func(t *testing.T) {
		t.Run("Name_ReturnsNonEmpty", func(t *testing.T) {
			p := factory(t, "")
			if p.Name() == "" {
				t.Error("Name() returned empty string")
			}
		})

		t.Run("Type_ReturnsNonEmpty", func(t *testing.T) {
			p := factory(t, "")
			if p.Type() == "" {
				t.Error("Type() returned empty string")
			}
		})

		t.Run("Capabilities_ReturnsSupportedTypes", func(t *testing.T) {
			p := factory(t, "")
			caps := p.Capabilities()
			if len(caps.SupportedRecordTypes) == 0 {
				t.Error("Capabilities().SupportedRecordTypes is empty; providers must support at least one record type")
			}
		})

		t.Run("List_WithCancelledContext", func(t *testing.T) {
			p := factory(t, "")
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err := p.List(ctx)
			// Most providers should return an error with canceled context.
			// We don't enforce this because some mock-based providers may not check ctx.
			// Just verify it doesn't panic.
			_ = err
		})

		t.Run("Create_WithCancelledContext", func(t *testing.T) {
			p := factory(t, "")
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			record := ARecord("conformance-test.example.com", "10.0.0.1")
			err := p.Create(ctx, record)
			_ = err // Don't enforce — just verify no panic
		})

		t.Run("Delete_WithCancelledContext", func(t *testing.T) {
			p := factory(t, "")
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			record := ARecord("conformance-test.example.com", "10.0.0.1")
			err := p.Delete(ctx, record)
			_ = err // Don't enforce — just verify no panic
		})

		t.Run("Updater_InterfaceCheck", func(t *testing.T) {
			p := factory(t, "")
			caps := p.Capabilities()
			if caps.SupportsNativeUpdate {
				_, ok := p.(provider.Updater)
				if !ok {
					t.Error("Capabilities().SupportsNativeUpdate is true but provider does not implement Updater interface")
				}
			}
		})
	})
}

// RunProviderCRUDConformance runs Create/List/Delete round-trip tests.
// The factory must return a provider connected to a working mock server
// that correctly handles CRUD operations.
//
// This is a stronger test than RunProviderConformance and requires a fully
// functional mock server (not just "doesn't panic" checks).
func RunProviderCRUDConformance(t *testing.T, name string, factory ProviderFactory, serverURL string) {
	t.Helper()
	t.Run("CRUD/"+name, func(t *testing.T) {
		ctx := context.Background()
		p := factory(t, serverURL)

		t.Run("Ping_Succeeds", func(t *testing.T) {
			err := p.Ping(ctx)
			RequireNoError(t, err)
		})

		t.Run("List_Initially", func(t *testing.T) {
			records, err := p.List(ctx)
			RequireNoError(t, err)
			// Records may or may not be empty depending on mock; just check no error.
			_ = records
		})

		t.Run("Create_ThenDelete", func(t *testing.T) {
			// Find a supported record type
			caps := p.Capabilities()
			if len(caps.SupportedRecordTypes) == 0 {
				t.Skip("no supported record types")
			}

			var record provider.Record
			switch caps.SupportedRecordTypes[0] {
			case provider.RecordTypeA:
				record = ARecord("crud-test.example.com", "10.0.0.99")
			case provider.RecordTypeAAAA:
				record = AAAARecord("crud-test.example.com", "fd00::99")
			case provider.RecordTypeCNAME:
				record = CNAMERecord("crud-test.example.com", "target.example.com")
			case provider.RecordTypeTXT:
				record = TXTRecord("crud-test.example.com", "v=test")
			default:
				record = ARecord("crud-test.example.com", "10.0.0.99")
			}

			err := p.Create(ctx, record)
			RequireNoError(t, err)

			err = p.Delete(ctx, record)
			RequireNoError(t, err)
		})
	})
}
