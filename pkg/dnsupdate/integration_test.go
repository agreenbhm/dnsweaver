//go:build integration

package dnsupdate_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/maxfield-allison/dnsweaver/pkg/dnsupdate"
)

// Integration test configuration loaded from environment variables.
// Required:
//
//	DNSUPDATE_TEST_SERVER   - DNS server address (host or host:port)
//	DNSUPDATE_TEST_ZONE     - Zone to use for tests (must allow dynamic updates)
//
// Optional (TSIG):
//
//	DNSUPDATE_TEST_TSIG_NAME      - TSIG key name (e.g., "dnsweaver.")
//	DNSUPDATE_TEST_TSIG_SECRET    - TSIG shared secret (base64)
//	DNSUPDATE_TEST_TSIG_ALGORITHM - TSIG algorithm (default: hmac-sha256)
//
// Optional:
//
//	DNSUPDATE_TEST_TCP            - Use TCP transport ("true" to enable)

// testConfig returns a validated Config from environment variables or skips the test.
func testConfig(t *testing.T) *dnsupdate.Config {
	t.Helper()

	server := os.Getenv("DNSUPDATE_TEST_SERVER")
	zone := os.Getenv("DNSUPDATE_TEST_ZONE")

	if server == "" || zone == "" {
		t.Skip("DNSUPDATE_TEST_SERVER and DNSUPDATE_TEST_ZONE must be set")
	}

	// Ensure zone has trailing dot.
	if !strings.HasSuffix(zone, ".") {
		zone += "."
	}

	cfg := &dnsupdate.Config{
		Server:  server,
		Zone:    zone,
		Timeout: 10 * time.Second,
		UseTCP:  os.Getenv("DNSUPDATE_TEST_TCP") == "true",
	}

	if name := os.Getenv("DNSUPDATE_TEST_TSIG_NAME"); name != "" {
		if !strings.HasSuffix(name, ".") {
			name += "."
		}
		cfg.TSIGKeyName = name
		cfg.TSIGSecret = os.Getenv("DNSUPDATE_TEST_TSIG_SECRET")
		cfg.TSIGAlgorithm = os.Getenv("DNSUPDATE_TEST_TSIG_ALGORITHM")
		if cfg.TSIGAlgorithm == "" {
			cfg.TSIGAlgorithm = "hmac-sha256"
		}
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("invalid test config: %v", err)
	}

	return cfg
}

// testClient returns a connected client, failing the test if creation fails.
func testClient(t *testing.T) *dnsupdate.Client {
	t.Helper()
	cfg := testConfig(t)
	client, err := dnsupdate.NewClient(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

// uniqueName returns a test-unique FQDN within the configured zone.
func uniqueName(t *testing.T, cfg *dnsupdate.Config, prefix string) string {
	t.Helper()
	// Use nanosecond timestamp for uniqueness across parallel runs.
	return fmt.Sprintf("%s-%d.%s", prefix, time.Now().UnixNano(), cfg.Zone)
}

// cleanupRecord deletes a record during test cleanup, logging but not failing on error.
func cleanupRecord(t *testing.T, client *dnsupdate.Client, rec dnsupdate.Record) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Delete(ctx, rec); err != nil {
		t.Logf("cleanup: failed to delete %s %s %s: %v", rec.Name, rec.TypeString(), rec.RData, err)
	}
}

// cleanupAllByType deletes all records of a type for a name during cleanup.
func cleanupAllByType(t *testing.T, client *dnsupdate.Client, name string, rtype uint16) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.DeleteAll(ctx, name, rtype); err != nil {
		t.Logf("cleanup: failed to delete all %s %s: %v", name, dns.TypeToString[rtype], err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Connectivity Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegration_Ping(t *testing.T) {
	client := testClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestIntegration_PingCanceledContext(t *testing.T) {
	client := testClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately canceled

	err := client.Ping(ctx)
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}

func TestIntegration_PingUnreachableServer(t *testing.T) {
	cfg := testConfig(t)
	// Point at a non-routable address to guarantee failure.
	cfg.Server = "192.0.2.1:65535" // RFC 5737 TEST-NET, unreachable
	cfg.Timeout = 2 * time.Second
	// Clear TSIG so Validate() doesn't fail on key name.
	cfg.TSIGKeyName = ""
	cfg.TSIGSecret = ""
	cfg.TSIGAlgorithm = ""

	client, err := dnsupdate.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.Ping(ctx)
	if err == nil {
		t.Fatal("expected error pinging unreachable server, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Record Create / Query Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegration_CreateAndQueryA(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "a-test")
	rec := dnsupdate.NewARecord(name, "198.51.100.10", 300)
	t.Cleanup(func() { cleanupRecord(t, client, rec) })

	// Create
	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create A: %v", err)
	}

	// Query
	records, err := client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Query A: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one A record, got none")
	}

	found := false
	for _, r := range records {
		if r.RData == "198.51.100.10" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected rdata 198.51.100.10, got %v", records)
	}
}

func TestIntegration_CreateAndQueryAAAA(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "aaaa-test")
	rec := dnsupdate.NewAAAARecord(name, "2001:db8::1", 300)
	t.Cleanup(func() { cleanupRecord(t, client, rec) })

	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create AAAA: %v", err)
	}

	records, err := client.Query(ctx, name, dns.TypeAAAA)
	if err != nil {
		t.Fatalf("Query AAAA: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one AAAA record, got none")
	}

	found := false
	for _, r := range records {
		if r.RData == "2001:db8::1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected rdata 2001:db8::1, got %v", records)
	}
}

func TestIntegration_CreateAndQueryCNAME(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "cname-test")
	rec := dnsupdate.NewCNAMERecord(name, "target.example.com.", 300)
	t.Cleanup(func() { cleanupRecord(t, client, rec) })

	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create CNAME: %v", err)
	}

	records, err := client.Query(ctx, name, dns.TypeCNAME)
	if err != nil {
		t.Fatalf("Query CNAME: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected CNAME record, got none")
	}
	if records[0].RData != "target.example.com." {
		t.Errorf("expected target.example.com., got %s", records[0].RData)
	}
}

func TestIntegration_CreateAndQueryTXT(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "txt-test")
	rec := dnsupdate.NewTXTRecord(name, "heritage=dnsweaver", 300)
	t.Cleanup(func() { cleanupRecord(t, client, rec) })

	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create TXT: %v", err)
	}

	records, err := client.Query(ctx, name, dns.TypeTXT)
	if err != nil {
		t.Fatalf("Query TXT: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected TXT record, got none")
	}
	if records[0].RData != "heritage=dnsweaver" {
		t.Errorf("expected heritage=dnsweaver, got %s", records[0].RData)
	}
}

func TestIntegration_CreateAndQueryMX(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "mx-test")
	rec := dnsupdate.NewMXRecord(name, "mail.example.com.", 10, 300)
	t.Cleanup(func() { cleanupRecord(t, client, rec) })

	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create MX: %v", err)
	}

	records, err := client.Query(ctx, name, dns.TypeMX)
	if err != nil {
		t.Fatalf("Query MX: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected MX record, got none")
	}
	if records[0].Priority != 10 {
		t.Errorf("expected priority 10, got %d", records[0].Priority)
	}
}

func TestIntegration_CreateAndQuerySRV(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "_sip._tcp.srv-test")
	rec := dnsupdate.NewSRVRecord(name, "sip.example.com.", 10, 20, 5060, 300)
	t.Cleanup(func() { cleanupRecord(t, client, rec) })

	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create SRV: %v", err)
	}

	records, err := client.Query(ctx, name, dns.TypeSRV)
	if err != nil {
		t.Fatalf("Query SRV: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected SRV record, got none")
	}

	r := records[0]
	if r.Priority != 10 || r.Weight != 20 || r.Port != 5060 {
		t.Errorf("SRV fields mismatch: priority=%d weight=%d port=%d", r.Priority, r.Weight, r.Port)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Update Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegration_UpdateARecord(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "update-a")
	oldRec := dnsupdate.NewARecord(name, "198.51.100.20", 300)
	newRec := dnsupdate.NewARecord(name, "198.51.100.21", 300)
	t.Cleanup(func() {
		cleanupRecord(t, client, oldRec)
		cleanupRecord(t, client, newRec)
	})

	// Create initial record.
	if err := client.Create(ctx, oldRec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Update to new target.
	if err := client.Update(ctx, oldRec, newRec); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Verify new value.
	records, err := client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	found := false
	for _, r := range records {
		if r.RData == "198.51.100.21" {
			found = true
		}
		if r.RData == "198.51.100.20" {
			t.Error("old record still present after update")
		}
	}
	if !found {
		t.Errorf("updated record not found, got %v", records)
	}
}

func TestIntegration_UpdateTTL(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "update-ttl")
	oldRec := dnsupdate.NewARecord(name, "198.51.100.30", 300)
	newRec := dnsupdate.NewARecord(name, "198.51.100.30", 600) // same rdata, different TTL
	t.Cleanup(func() {
		cleanupAllByType(t, client, name, dns.TypeA)
	})

	if err := client.Create(ctx, oldRec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := client.Update(ctx, oldRec, newRec); err != nil {
		t.Fatalf("Update TTL: %v", err)
	}

	records, err := client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected record after TTL update, got none")
	}
	if records[0].TTL != 600 {
		t.Errorf("expected TTL 600, got %d", records[0].TTL)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Delete Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegration_DeleteSpecificRecord(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "delete-specific")
	rec := dnsupdate.NewARecord(name, "198.51.100.40", 300)

	// Create
	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Delete
	if err := client.Delete(ctx, rec); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify gone
	records, err := client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Query after delete: %v", err)
	}
	for _, r := range records {
		if r.RData == "198.51.100.40" {
			t.Error("record still exists after Delete")
		}
	}
}

func TestIntegration_DeleteAllByType(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "delete-all")

	// Create two A records for the same name.
	rec1 := dnsupdate.NewARecord(name, "198.51.100.50", 300)
	rec2 := dnsupdate.NewARecord(name, "198.51.100.51", 300)

	if err := client.Create(ctx, rec1); err != nil {
		t.Fatalf("Create rec1: %v", err)
	}
	if err := client.Create(ctx, rec2); err != nil {
		t.Fatalf("Create rec2: %v", err)
	}

	// Verify both exist.
	records, err := client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Query before DeleteAll: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected 2 A records, got %d", len(records))
	}

	// DeleteAll
	if err := client.DeleteAll(ctx, name, dns.TypeA); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}

	// Verify all gone.
	records, err = client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Query after DeleteAll: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records after DeleteAll, got %d", len(records))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Error Handling Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegration_CreateOutOfZone(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	rec := dnsupdate.NewARecord("host.wrong-zone.example.", "198.51.100.60", 300)
	err := client.Create(ctx, rec)
	if err == nil {
		t.Fatal("expected error creating record outside zone, got nil")
	}
}

func TestIntegration_ZoneMismatchDelete(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	rec := dnsupdate.NewARecord("host.wrong-zone.example.", "198.51.100.70", 300)
	err := client.Delete(ctx, rec)
	if err == nil {
		t.Fatal("expected error deleting record outside zone, got nil")
	}
}

func TestIntegration_ZoneMismatchDeleteAll(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	err := client.DeleteAll(ctx, "host.wrong-zone.example.", dns.TypeA)
	if err == nil {
		t.Fatal("expected error for DeleteAll outside zone, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Edge Cases
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegration_MultipleRecordsSameName(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "multi-a")
	rec1 := dnsupdate.NewARecord(name, "198.51.100.80", 300)
	rec2 := dnsupdate.NewARecord(name, "198.51.100.81", 300)
	t.Cleanup(func() { cleanupAllByType(t, client, name, dns.TypeA) })

	if err := client.Create(ctx, rec1); err != nil {
		t.Fatalf("Create rec1: %v", err)
	}
	if err := client.Create(ctx, rec2); err != nil {
		t.Fatalf("Create rec2: %v", err)
	}

	records, err := client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(records) < 2 {
		t.Errorf("expected at least 2 A records, got %d", len(records))
	}
}

func TestIntegration_FullLifecycle(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "lifecycle")
	t.Cleanup(func() { cleanupAllByType(t, client, name, dns.TypeA) })

	// 1. Create
	rec := dnsupdate.NewARecord(name, "198.51.100.90", 300)
	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Step 1 (Create): %v", err)
	}

	// 2. Query — verify exists.
	records, err := client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Step 2 (Query): %v", err)
	}
	if len(records) == 0 {
		t.Fatal("Step 2: expected record after create")
	}

	// 3. Update target.
	newRec := dnsupdate.NewARecord(name, "198.51.100.91", 300)
	if err := client.Update(ctx, rec, newRec); err != nil {
		t.Fatalf("Step 3 (Update): %v", err)
	}

	// 4. Query — verify updated.
	records, err = client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Step 4 (Query): %v", err)
	}
	found := false
	for _, r := range records {
		if r.RData == "198.51.100.91" {
			found = true
		}
		if r.RData == "198.51.100.90" {
			t.Error("Step 4: old record still exists")
		}
	}
	if !found {
		t.Error("Step 4: updated record not found")
	}

	// 5. Delete.
	if err := client.Delete(ctx, newRec); err != nil {
		t.Fatalf("Step 5 (Delete): %v", err)
	}

	// 6. Query — verify gone.
	records, err = client.Query(ctx, name, dns.TypeA)
	if err != nil {
		t.Fatalf("Step 6 (Query): %v", err)
	}
	for _, r := range records {
		if r.RData == "198.51.100.91" {
			t.Error("Step 6: record still exists after delete")
		}
	}
}

func TestIntegration_LongTXTRecord(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	name := uniqueName(t, cfg, "long-txt")
	// Create a TXT value near the 255-char string limit.
	longValue := strings.Repeat("a", 250)
	rec := dnsupdate.NewTXTRecord(name, longValue, 300)
	t.Cleanup(func() { cleanupRecord(t, client, rec) })

	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create long TXT: %v", err)
	}

	records, err := client.Query(ctx, name, dns.TypeTXT)
	if err != nil {
		t.Fatalf("Query long TXT: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected long TXT record, got none")
	}
	if records[0].RData != longValue {
		t.Errorf("TXT rdata mismatch: expected length %d, got length %d", len(longValue), len(records[0].RData))
	}
}

func TestIntegration_CreateContextTimeout(t *testing.T) {
	cfg := testConfig(t)
	// Use an unreachable server so the create blocks until timeout.
	cfg.Server = "192.0.2.1:65535"
	cfg.Timeout = 1 * time.Second
	cfg.TSIGKeyName = ""
	cfg.TSIGSecret = ""
	cfg.TSIGAlgorithm = ""

	client, err := dnsupdate.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rec := dnsupdate.NewARecord("timeout-test."+cfg.Zone, "198.51.100.99", 300)
	err = client.Create(ctx, rec)
	if err == nil {
		t.Fatal("expected error from timed-out create, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// AXFR Tests (may be restricted by server policy)
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegration_ListByAXFR(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	// Seed a record so the zone is non-empty.
	name := uniqueName(t, cfg, "axfr-seed")
	rec := dnsupdate.NewARecord(name, "198.51.100.100", 300)
	t.Cleanup(func() { cleanupRecord(t, client, rec) })

	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create seed: %v", err)
	}

	records, err := client.ListByAXFR(ctx)
	if err != nil {
		// AXFR may be restricted; skip rather than fail.
		t.Skipf("AXFR not available: %v", err)
	}

	if len(records) == 0 {
		t.Error("AXFR returned no records, expected at least the seed record")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Client Metadata Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegration_ClientZoneAndServer(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)

	if client.Zone() != cfg.Zone {
		t.Errorf("Zone() = %s, want %s", client.Zone(), cfg.Zone)
	}

	if client.Server() == "" {
		t.Error("Server() returned empty string")
	}
}

func TestIntegration_LastUpdateUpdatedAfterCreate(t *testing.T) {
	cfg := testConfig(t)
	client := testClient(t)
	ctx := context.Background()

	before := client.LastUpdate()

	name := uniqueName(t, cfg, "last-update")
	rec := dnsupdate.NewARecord(name, "198.51.100.110", 300)
	t.Cleanup(func() { cleanupRecord(t, client, rec) })

	if err := client.Create(ctx, rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	after := client.LastUpdate()
	if !after.After(before) {
		t.Errorf("LastUpdate not advanced: before=%v, after=%v", before, after)
	}
}

func TestIntegration_CloseIsIdempotent(t *testing.T) {
	client := testClient(t)
	if err := client.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}
