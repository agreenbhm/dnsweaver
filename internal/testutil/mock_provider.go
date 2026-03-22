// Package testutil provides shared test utilities, mock implementations,
// and assertion helpers for dnsweaver tests.
//
// This package exports reusable mocks for provider.Provider, source.Source,
// and workload.Lister interfaces, along with record builders, a mock HTTP
// server with request recording, and provider-specific response builders.
//
// All mock types are safe for concurrent use.
package testutil

import (
	"context"
	"sync"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// MockProvider implements provider.Provider for testing.
// It tracks all Create/Delete/Update calls for verification.
// Thread-safe via internal mutex.
type MockProvider struct {
	name     string
	typeName string
	caps     provider.Capabilities

	mu       sync.Mutex
	records  []provider.Record
	created  []provider.Record
	deleted  []provider.Record
	updated  []UpdateCall
	pingErr  error
	listErr  error
	createFn func(ctx context.Context, r provider.Record) error
	deleteFn func(ctx context.Context, r provider.Record) error
	updateFn func(ctx context.Context, existing, desired provider.Record) error
}

// UpdateCall records an Update operation for verification.
type UpdateCall struct {
	Existing provider.Record
	Desired  provider.Record
}

// NewMockProvider creates a new MockProvider with sensible defaults.
// Supports all record types and ownership TXT by default.
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:     name,
		typeName: "mock",
		caps: provider.Capabilities{
			SupportsOwnershipTXT: true,
			SupportsNativeUpdate: true,
			SupportedRecordTypes: []provider.RecordType{
				provider.RecordTypeA,
				provider.RecordTypeAAAA,
				provider.RecordTypeCNAME,
				provider.RecordTypeSRV,
				provider.RecordTypeTXT,
			},
		},
		records: make([]provider.Record, 0),
		created: make([]provider.Record, 0),
		deleted: make([]provider.Record, 0),
		updated: make([]UpdateCall, 0),
	}
}

// Name returns the provider instance name.
func (m *MockProvider) Name() string { return m.name }

// Type returns the provider type.
func (m *MockProvider) Type() string { return m.typeName }

// Capabilities returns the provider's feature support.
func (m *MockProvider) Capabilities() provider.Capabilities { return m.caps }

// Ping checks connectivity (returns configured error or nil).
func (m *MockProvider) Ping(_ context.Context) error {
	return m.pingErr
}

// List returns all records (or configured error).
func (m *MockProvider) List(_ context.Context) ([]provider.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]provider.Record, len(m.records))
	copy(result, m.records)
	return result, nil
}

// Create adds a record. Calls custom createFn if configured.
func (m *MockProvider) Create(ctx context.Context, r provider.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.createFn != nil {
		if err := m.createFn(ctx, r); err != nil {
			return err
		}
	}

	m.created = append(m.created, r)
	m.records = append(m.records, r)
	return nil
}

// Delete removes a record. Calls custom deleteFn if configured.
func (m *MockProvider) Delete(ctx context.Context, r provider.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deleteFn != nil {
		if err := m.deleteFn(ctx, r); err != nil {
			return err
		}
	}

	m.deleted = append(m.deleted, r)
	newRecords := make([]provider.Record, 0, len(m.records))
	for _, rec := range m.records {
		if rec.Hostname != r.Hostname || rec.Type != r.Type || rec.Target != r.Target {
			newRecords = append(newRecords, rec)
		}
	}
	m.records = newRecords
	return nil
}

// Update modifies a record in place. Calls custom updateFn if configured.
// Implements the provider.Updater interface.
func (m *MockProvider) Update(ctx context.Context, existing, desired provider.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.updateFn != nil {
		if err := m.updateFn(ctx, existing, desired); err != nil {
			return err
		}
	}

	m.updated = append(m.updated, UpdateCall{Existing: existing, Desired: desired})

	// Replace existing record in the store
	for i, rec := range m.records {
		if rec.Hostname == existing.Hostname && rec.Type == existing.Type && rec.Target == existing.Target {
			m.records[i] = desired
			return nil
		}
	}
	return nil
}

// --- Configuration methods ---

// SetType overrides the provider type name.
func (m *MockProvider) SetType(typeName string) {
	m.typeName = typeName
}

// SetCapabilities overrides the default capabilities.
func (m *MockProvider) SetCapabilities(caps provider.Capabilities) {
	m.caps = caps
}

// SetPingError configures Ping() to return the given error.
func (m *MockProvider) SetPingError(err error) {
	m.pingErr = err
}

// SetListError configures List() to return the given error.
func (m *MockProvider) SetListError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listErr = err
}

// SetCreateFunc configures a custom function called on Create().
func (m *MockProvider) SetCreateFunc(fn func(ctx context.Context, r provider.Record) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createFn = fn
}

// SetDeleteFunc configures a custom function called on Delete().
func (m *MockProvider) SetDeleteFunc(fn func(ctx context.Context, r provider.Record) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteFn = fn
}

// SetUpdateFunc configures a custom function called on Update().
func (m *MockProvider) SetUpdateFunc(fn func(ctx context.Context, existing, desired provider.Record) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateFn = fn
}

// AddRecord adds a record to the provider's current record store.
func (m *MockProvider) AddRecord(r provider.Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, r)
}

// SetRecords replaces the provider's entire record store.
func (m *MockProvider) SetRecords(records ...provider.Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = make([]provider.Record, len(records))
	copy(m.records, records)
}

// --- Verification methods ---

// Created returns all records that were passed to Create().
func (m *MockProvider) Created() []provider.Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]provider.Record, len(m.created))
	copy(result, m.created)
	return result
}

// Deleted returns all records that were passed to Delete().
func (m *MockProvider) Deleted() []provider.Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]provider.Record, len(m.deleted))
	copy(result, m.deleted)
	return result
}

// Updated returns all Update calls recorded.
func (m *MockProvider) Updated() []UpdateCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]UpdateCall, len(m.updated))
	copy(result, m.updated)
	return result
}

// CreatedDNSRecords returns only non-TXT records that were created.
func (m *MockProvider) CreatedDNSRecords() []provider.Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []provider.Record
	for _, r := range m.created {
		if r.Type != provider.RecordTypeTXT {
			result = append(result, r)
		}
	}
	return result
}

// CreatedOwnershipRecords returns only TXT records that were created.
func (m *MockProvider) CreatedOwnershipRecords() []provider.Record {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []provider.Record
	for _, r := range m.created {
		if r.Type == provider.RecordTypeTXT {
			result = append(result, r)
		}
	}
	return result
}

// Reset clears all tracked records, created, deleted, and updated calls.
func (m *MockProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = make([]provider.Record, 0)
	m.created = make([]provider.Record, 0)
	m.deleted = make([]provider.Record, 0)
	m.updated = make([]UpdateCall, 0)
}

// Compile-time interface checks.
var (
	_ provider.Provider = (*MockProvider)(nil)
	_ provider.Updater  = (*MockProvider)(nil)
)
