package testutil_test

import (
	"testing"

	"github.com/maxfield-allison/dnsweaver/internal/testutil"
	"github.com/maxfield-allison/dnsweaver/pkg/provider"
)

func TestRecordBuilders(t *testing.T) {
	t.Run("ARecord", func(t *testing.T) {
		r := testutil.ARecord("app.example.com", "10.0.0.1")
		testutil.AssertEqual(t, "hostname", r.Hostname, "app.example.com")
		testutil.AssertEqual(t, "type", r.Type, provider.RecordTypeA)
		testutil.AssertEqual(t, "target", r.Target, "10.0.0.1")
		testutil.AssertEqual(t, "ttl", r.TTL, 300)
	})

	t.Run("AAAARecord", func(t *testing.T) {
		r := testutil.AAAARecord("app.example.com", "fd00::1")
		testutil.AssertEqual(t, "type", r.Type, provider.RecordTypeAAAA)
		testutil.AssertEqual(t, "target", r.Target, "fd00::1")
	})

	t.Run("CNAMERecord", func(t *testing.T) {
		r := testutil.CNAMERecord("www.example.com", "app.example.com")
		testutil.AssertEqual(t, "type", r.Type, provider.RecordTypeCNAME)
		testutil.AssertEqual(t, "target", r.Target, "app.example.com")
	})

	t.Run("TXTRecord", func(t *testing.T) {
		r := testutil.TXTRecord("_dnsweaver.app.example.com", "heritage=dnsweaver")
		testutil.AssertEqual(t, "type", r.Type, provider.RecordTypeTXT)
		testutil.AssertEqual(t, "target", r.Target, "heritage=dnsweaver")
	})

	t.Run("SRVRecord", func(t *testing.T) {
		r := testutil.SRVRecord("_http._tcp.example.com", "server.example.com", 8080, 10, 20)
		testutil.AssertEqual(t, "type", r.Type, provider.RecordTypeSRV)
		if r.SRV == nil {
			t.Fatal("SRV data should not be nil")
		}
		testutil.AssertEqual(t, "port", r.SRV.Port, uint16(8080))
		testutil.AssertEqual(t, "priority", r.SRV.Priority, uint16(10))
		testutil.AssertEqual(t, "weight", r.SRV.Weight, uint16(20))
	})

	t.Run("OwnershipRecord", func(t *testing.T) {
		r := testutil.OwnershipRecord("app.example.com", "instance-1")
		testutil.AssertEqual(t, "hostname", r.Hostname, "_dnsweaver.app.example.com")
		testutil.AssertEqual(t, "type", r.Type, provider.RecordTypeTXT)
		testutil.AssertContains(t, r.Target, "heritage=dnsweaver")
		testutil.AssertContains(t, r.Target, "instance=instance-1")
	})

	t.Run("ARecordWithTTL", func(t *testing.T) {
		r := testutil.ARecordWithTTL("app.example.com", "10.0.0.1", 3600)
		testutil.AssertEqual(t, "ttl", r.TTL, 3600)
	})

	t.Run("RecordWithID", func(t *testing.T) {
		r := testutil.RecordWithID(testutil.ARecord("test.example.com", "1.2.3.4"), "rec-123")
		testutil.AssertEqual(t, "providerID", r.ProviderID, "rec-123")
	})

	t.Run("RecordWithMeta", func(t *testing.T) {
		meta := map[string]string{"proxied": "true"}
		r := testutil.RecordWithMeta(testutil.ARecord("test.example.com", "1.2.3.4"), meta)
		if r.Metadata == nil {
			t.Fatal("metadata should not be nil")
		}
		testutil.AssertEqual(t, "proxied", r.Metadata["proxied"], "true")
	})
}

func TestAssertions(t *testing.T) {
	records := []provider.Record{
		testutil.ARecord("app.example.com", "10.0.0.1"),
		testutil.CNAMERecord("www.example.com", "app.example.com"),
		testutil.OwnershipRecord("app.example.com", ""),
	}

	t.Run("AssertRecordExists", func(t *testing.T) {
		testutil.AssertRecordExists(t, records, "app.example.com", provider.RecordTypeA)
		testutil.AssertRecordExists(t, records, "www.example.com", provider.RecordTypeCNAME)
	})

	t.Run("AssertRecordNotExists", func(t *testing.T) {
		testutil.AssertRecordNotExists(t, records, "missing.example.com", provider.RecordTypeA)
	})

	t.Run("FindRecord", func(t *testing.T) {
		r := testutil.FindRecord(records, "app.example.com", provider.RecordTypeA)
		if r == nil {
			t.Fatal("expected to find record")
		}
		testutil.AssertEqual(t, "target", r.Target, "10.0.0.1")
	})

	t.Run("FindRecord_NotFound", func(t *testing.T) {
		r := testutil.FindRecord(records, "missing.example.com", provider.RecordTypeA)
		if r != nil {
			t.Error("expected nil for missing record")
		}
	})

	t.Run("FindRecordByTarget", func(t *testing.T) {
		r := testutil.FindRecordByTarget(records, "app.example.com", provider.RecordTypeA, "10.0.0.1")
		if r == nil {
			t.Fatal("expected to find record by target")
		}
	})
}

func TestConformance_MockProvider(t *testing.T) {
	testutil.RunProviderConformance(t, "MockProvider", func(t *testing.T, _ string) provider.Provider {
		t.Helper()
		return testutil.NewMockProvider("conformance-mock")
	})
}
