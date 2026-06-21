//go:build integration

// These tests run against a REAL PowerDNS Authoritative server and are excluded
// from the normal `go test` run by the `integration` build tag.
//
// Usage:
//
//	PDNS_URL=http://localhost:8081 PDNS_KEY=secret123 PDNS_ZONE=example.test \
//	    go test -tags integration -v ./providers/powerdns/
//
// The configured zone must already exist on the server (the provider never
// creates zones). The test cleans up the records it creates.
package powerdns

import (
	"context"
	"os"
	"testing"

	"github.com/maxfield-allison/dnsweaver/pkg/provider"
)

func itProvider(t *testing.T) (*Provider, string) {
	t.Helper()
	url, key, zone := os.Getenv("PDNS_URL"), os.Getenv("PDNS_KEY"), os.Getenv("PDNS_ZONE")
	if url == "" || key == "" || zone == "" {
		t.Skip("set PDNS_URL, PDNS_KEY, PDNS_ZONE to run integration tests")
	}
	p, err := New("pdns-it", &Config{URL: url, APIKey: key, Zone: zone, ServerID: "localhost", TTL: 120})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p, zone
}

func find(t *testing.T, p *Provider, host string, rt provider.RecordType, target string) *provider.Record {
	t.Helper()
	recs, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for i := range recs {
		if recs[i].Hostname == host && recs[i].Type == rt && recs[i].Target == target {
			return &recs[i]
		}
	}
	return nil
}

func count(t *testing.T, p *Provider, host string, rt provider.RecordType) int {
	t.Helper()
	recs, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	n := 0
	for _, r := range recs {
		if r.Hostname == host && r.Type == rt {
			n++
		}
	}
	return n
}

func TestIntegration_Lifecycle(t *testing.T) {
	ctx := context.Background()
	p, zone := itProvider(t)

	if err := p.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	aHost := "it-a." + zone
	txtHost := "it-txt." + zone
	cnameHost := "it-cname." + zone
	srvHost := "_sip._tcp.it." + zone
	srvData := &provider.SRVData{Priority: 10, Weight: 20, Port: 5060}

	cleanup := func() {
		_ = p.Delete(ctx, provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.50"})
		_ = p.Delete(ctx, provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.51"})
		_ = p.Delete(ctx, provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.60"})
		_ = p.Delete(ctx, provider.Record{Hostname: txtHost, Type: provider.RecordTypeTXT, Target: "heritage=dnsweaver,instance=it"})
		_ = p.Delete(ctx, provider.Record{Hostname: cnameHost, Type: provider.RecordTypeCNAME, Target: "target." + zone})
		_ = p.Delete(ctx, provider.Record{Hostname: srvHost, Type: provider.RecordTypeSRV, Target: "sip." + zone, SRV: srvData})
	}
	cleanup()
	t.Cleanup(cleanup)

	t.Run("create A", func(t *testing.T) {
		if err := p.Create(ctx, provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.50"}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if find(t, p, aHost, provider.RecordTypeA, "192.0.2.50") == nil {
			t.Fatal("A 192.0.2.50 not found after create")
		}
	})

	t.Run("round-robin sibling preserved", func(t *testing.T) {
		if err := p.Create(ctx, provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.51"}); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if n := count(t, p, aHost, provider.RecordTypeA); n != 2 {
			t.Fatalf("expected 2 A records (round-robin), got %d", n)
		}
	})

	t.Run("idempotent re-create", func(t *testing.T) {
		if err := p.Create(ctx, provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.51"}); err != nil {
			t.Fatalf("Create (idempotent): %v", err)
		}
		if n := count(t, p, aHost, provider.RecordTypeA); n != 2 {
			t.Fatalf("re-create changed count: got %d, want 2", n)
		}
	})

	t.Run("native update preserves sibling", func(t *testing.T) {
		err := p.Update(ctx,
			provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.50"},
			provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.60"},
		)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if find(t, p, aHost, provider.RecordTypeA, "192.0.2.60") == nil {
			t.Fatal("updated A 192.0.2.60 not found")
		}
		if find(t, p, aHost, provider.RecordTypeA, "192.0.2.50") != nil {
			t.Fatal("old A 192.0.2.50 still present after update")
		}
		if find(t, p, aHost, provider.RecordTypeA, "192.0.2.51") == nil {
			t.Fatal("sibling A 192.0.2.51 lost during update")
		}
	})

	t.Run("TXT round-trips unquoted", func(t *testing.T) {
		val := "heritage=dnsweaver,instance=it"
		if err := p.Create(ctx, provider.Record{Hostname: txtHost, Type: provider.RecordTypeTXT, Target: val}); err != nil {
			t.Fatalf("Create TXT: %v", err)
		}
		if find(t, p, txtHost, provider.RecordTypeTXT, val) == nil {
			t.Fatalf("TXT did not round-trip unquoted as %q", val)
		}
	})

	t.Run("CNAME round-trips bare (no trailing dot)", func(t *testing.T) {
		target := "target." + zone
		if err := p.Create(ctx, provider.Record{Hostname: cnameHost, Type: provider.RecordTypeCNAME, Target: target}); err != nil {
			t.Fatalf("Create CNAME: %v", err)
		}
		if find(t, p, cnameHost, provider.RecordTypeCNAME, target) == nil {
			t.Fatalf("CNAME did not round-trip as bare %q", target)
		}
	})

	t.Run("SRV encode/parse", func(t *testing.T) {
		rec := provider.Record{Hostname: srvHost, Type: provider.RecordTypeSRV, Target: "sip." + zone, SRV: srvData}
		if err := p.Create(ctx, rec); err != nil {
			t.Fatalf("Create SRV: %v", err)
		}
		got := find(t, p, srvHost, provider.RecordTypeSRV, "sip."+zone)
		if got == nil || got.SRV == nil {
			t.Fatal("SRV not found/parsed")
		}
		if got.SRV.Priority != 10 || got.SRV.Weight != 20 || got.SRV.Port != 5060 {
			t.Fatalf("SRV fields wrong: %+v", got.SRV)
		}
	})

	t.Run("delete one of rrset keeps sibling", func(t *testing.T) {
		if err := p.Delete(ctx, provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.60"}); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if find(t, p, aHost, provider.RecordTypeA, "192.0.2.60") != nil {
			t.Fatal("deleted A 192.0.2.60 still present")
		}
		if find(t, p, aHost, provider.RecordTypeA, "192.0.2.51") == nil {
			t.Fatal("sibling A 192.0.2.51 wrongly removed")
		}
	})

	t.Run("delete last empties rrset", func(t *testing.T) {
		if err := p.Delete(ctx, provider.Record{Hostname: aHost, Type: provider.RecordTypeA, Target: "192.0.2.51"}); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if n := count(t, p, aHost, provider.RecordTypeA); n != 0 {
			t.Fatalf("expected 0 A records after deleting all, got %d", n)
		}
	})

	t.Run("delete absent is a no-op", func(t *testing.T) {
		if err := p.Delete(ctx, provider.Record{Hostname: "nope." + zone, Type: provider.RecordTypeA, Target: "203.0.113.9"}); err != nil {
			t.Fatalf("Delete of absent record should be a no-op, got: %v", err)
		}
	})
}

func TestIntegration_Delete_PreservesSiblingTTL(t *testing.T) {
	ctx := context.Background()
	p, zone := itProvider(t)

	host := "it-ttl-del." + zone

	cleanup := func() {
		_ = p.Delete(ctx, provider.Record{Hostname: host, Type: provider.RecordTypeA, Target: "192.0.2.70", TTL: 600})
		_ = p.Delete(ctx, provider.Record{Hostname: host, Type: provider.RecordTypeA, Target: "192.0.2.71", TTL: 600})
	}
	cleanup()
	t.Cleanup(cleanup)

	// Create both records with TTL=600 so the rrset TTL is 600.
	if err := p.Create(ctx, provider.Record{Hostname: host, Type: provider.RecordTypeA, Target: "192.0.2.70", TTL: 600}); err != nil {
		t.Fatalf("Create 192.0.2.70: %v", err)
	}
	if err := p.Create(ctx, provider.Record{Hostname: host, Type: provider.RecordTypeA, Target: "192.0.2.71", TTL: 600}); err != nil {
		t.Fatalf("Create 192.0.2.71: %v", err)
	}

	// Delete one record with zero TTL — survivor's rrset TTL must stay 600.
	if err := p.Delete(ctx, provider.Record{Hostname: host, Type: provider.RecordTypeA, Target: "192.0.2.70"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// List reads TTL from the rrset; survivor must report TTL=600.
	rec := find(t, p, host, provider.RecordTypeA, "192.0.2.71")
	if rec == nil {
		t.Fatal("surviving record 192.0.2.71 not found after partial delete")
	}
	if rec.TTL != 600 {
		t.Errorf("survivor TTL = %d, want 600 (must be preserved from rrset)", rec.TTL)
	}
}

func TestIntegration_PingMissingZone(t *testing.T) {
	url, key := os.Getenv("PDNS_URL"), os.Getenv("PDNS_KEY")
	if url == "" || key == "" {
		t.Skip("set PDNS_URL, PDNS_KEY to run integration tests")
	}
	p, err := New("pdns-it-missing", &Config{URL: url, APIKey: key, Zone: "does-not-exist.invalid", ServerID: "localhost", TTL: 120})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := p.Ping(context.Background()); err == nil {
		t.Fatal("Ping should fail for a non-existent zone")
	}
}
