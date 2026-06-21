package testutil_test

import (
	"context"
	"errors"
	"testing"

	"github.com/maxfield-allison/dnsweaver/internal/testutil"
	"github.com/maxfield-allison/dnsweaver/pkg/provider"
)

func TestMockProvider_BasicOperations(t *testing.T) {
	p := testutil.NewMockProvider("test-dns")

	// Verify defaults
	testutil.AssertEqual(t, "Name", p.Name(), "test-dns")
	testutil.AssertEqual(t, "Type", p.Type(), "mock")

	// Ping should succeed by default
	testutil.RequireNoError(t, p.Ping(context.Background()))

	// List should return empty
	records, err := p.List(context.Background())
	testutil.RequireNoError(t, err)
	testutil.AssertLen(t, "initial records", records, 0)
}

func TestMockProvider_CRUD(t *testing.T) {
	ctx := context.Background()
	p := testutil.NewMockProvider("crud-test")

	rec := testutil.ARecord("app.example.com", "10.0.0.1")

	// Create
	testutil.RequireNoError(t, p.Create(ctx, rec))
	testutil.AssertLen(t, "created", p.Created(), 1)
	testutil.AssertLen(t, "records after create", must(p.List(ctx)), 1)

	// Delete
	testutil.RequireNoError(t, p.Delete(ctx, rec))
	testutil.AssertLen(t, "deleted", p.Deleted(), 1)
	testutil.AssertLen(t, "records after delete", must(p.List(ctx)), 0)
}

func TestMockProvider_CustomFunctions(t *testing.T) {
	ctx := context.Background()
	p := testutil.NewMockProvider("custom")

	createErr := errors.New("create failed")
	p.SetCreateFunc(func(_ context.Context, _ provider.Record) error {
		return createErr
	})

	err := p.Create(ctx, testutil.ARecord("test.example.com", "10.0.0.1"))
	testutil.RequireErrorContains(t, err, "create failed")
	testutil.AssertLen(t, "created (should be empty on error)", p.Created(), 0)
}

func TestMockProvider_ErrorConfiguration(t *testing.T) {
	p := testutil.NewMockProvider("error-test")

	p.SetPingError(errors.New("connection refused"))
	testutil.RequireErrorContains(t, p.Ping(context.Background()), "connection refused")

	p.SetListError(errors.New("timeout"))
	_, err := p.List(context.Background())
	testutil.RequireErrorContains(t, err, "timeout")
}

func TestMockProvider_Update(t *testing.T) {
	ctx := context.Background()
	p := testutil.NewMockProvider("update-test")

	old := testutil.ARecord("app.example.com", "10.0.0.1")
	p.AddRecord(old)

	newRec := testutil.ARecord("app.example.com", "10.0.0.2")
	testutil.RequireNoError(t, p.Update(ctx, old, newRec))

	updated := p.Updated()
	testutil.AssertLen(t, "updated", updated, 1)
	testutil.AssertEqual(t, "existing target", updated[0].Existing.Target, "10.0.0.1")
	testutil.AssertEqual(t, "desired target", updated[0].Desired.Target, "10.0.0.2")
}

func TestMockProvider_Reset(t *testing.T) {
	ctx := context.Background()
	p := testutil.NewMockProvider("reset-test")
	p.AddRecord(testutil.ARecord("a.example.com", "1.1.1.1"))
	_ = p.Create(ctx, testutil.ARecord("b.example.com", "2.2.2.2"))

	p.Reset()
	records, _ := p.List(context.Background())
	testutil.AssertLen(t, "records after reset", records, 0)
	testutil.AssertLen(t, "created after reset", p.Created(), 0)
	testutil.AssertLen(t, "deleted after reset", p.Deleted(), 0)
}

func TestMockProvider_InterfaceCompliance(t *testing.T) {
	p := testutil.NewMockProvider("iface-check")

	// Provider interface
	var _ provider.Provider = p

	// Updater interface
	var _ provider.Updater = p
}

func TestMockProvider_Capabilities(t *testing.T) {
	p := testutil.NewMockProvider("caps-test")
	caps := p.Capabilities()

	if !caps.SupportsOwnershipTXT {
		t.Error("expected SupportsOwnershipTXT to be true by default")
	}
	if !caps.SupportsNativeUpdate {
		t.Error("expected SupportsNativeUpdate to be true by default")
	}
	if len(caps.SupportedRecordTypes) != 5 {
		t.Errorf("expected 5 supported record types, got %d", len(caps.SupportedRecordTypes))
	}

	// Override capabilities
	p.SetCapabilities(provider.Capabilities{
		SupportsOwnershipTXT: false,
		SupportedRecordTypes: []provider.RecordType{provider.RecordTypeA},
	})
	caps = p.Capabilities()
	if caps.SupportsOwnershipTXT {
		t.Error("expected SupportsOwnershipTXT to be false after override")
	}
	testutil.AssertLen(t, "overridden types", caps.SupportedRecordTypes, 1)
}

// must is a test helper that returns the result and fails on error.
func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
