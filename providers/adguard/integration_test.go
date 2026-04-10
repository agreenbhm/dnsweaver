//go:build integration

package adguard

import (
	"context"
	"os"
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// TestIntegration_FullCRUD runs a full create/read/update/delete cycle against a live AdGuard Home instance.
//
// To run: go test -tags=integration -run TestIntegration ./providers/adguard/ -v
//
// Expects:
//   - ADGUARD_TEST_URL (default: http://localhost:3000)
//   - ADGUARD_TEST_USERNAME (default: admin)
//   - ADGUARD_TEST_PASSWORD (default: testpass123)
func TestIntegration_FullCRUD(t *testing.T) {
	url := envOr("ADGUARD_TEST_URL", "http://localhost:3000")
	username := envOr("ADGUARD_TEST_USERNAME", "admin")
	password := envOr("ADGUARD_TEST_PASSWORD", "testpass123")

	cfg := &Config{
		URL:      url,
		Username: username,
		Password: password,
		TTL:      300,
	}

	p, err := New("integration-test", cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()

	// 1. Ping
	t.Run("Ping", func(t *testing.T) {
		if err := p.Ping(ctx); err != nil {
			t.Fatalf("Ping() failed: %v", err)
		}
	})

	// 2. Create records
	testRecords := []provider.Record{
		{Hostname: "inttest-a.dnsweaver.local", Type: provider.RecordTypeA, Target: "10.99.99.1"},
		{Hostname: "inttest-aaaa.dnsweaver.local", Type: provider.RecordTypeAAAA, Target: "2001:db8:99::1"},
		{Hostname: "inttest-cname.dnsweaver.local", Type: provider.RecordTypeCNAME, Target: "inttest-a.dnsweaver.local"},
	}

	t.Run("Create", func(t *testing.T) {
		for _, rec := range testRecords {
			if err := p.Create(ctx, rec); err != nil {
				t.Errorf("Create(%s %s) failed: %v", rec.Hostname, rec.Type, err)
			}
		}
	})

	// 3. List and verify
	t.Run("List", func(t *testing.T) {
		records, err := p.List(ctx)
		if err != nil {
			t.Fatalf("List() failed: %v", err)
		}

		found := map[string]bool{}
		for _, r := range records {
			found[r.Hostname+":"+string(r.Type)+":"+r.Target] = true
		}

		for _, expected := range testRecords {
			key := expected.Hostname + ":" + string(expected.Type) + ":" + expected.Target
			if !found[key] {
				t.Errorf("List() missing record: %s", key)
			}
		}
	})

	// 4. Update
	t.Run("Update", func(t *testing.T) {
		existing := testRecords[0]
		desired := provider.Record{
			Hostname: "inttest-a.dnsweaver.local",
			Type:     provider.RecordTypeA,
			Target:   "10.99.99.2",
		}

		if err := p.Update(ctx, existing, desired); err != nil {
			t.Errorf("Update() failed: %v", err)
		}

		// Verify
		records, err := p.List(ctx)
		if err != nil {
			t.Fatalf("List() after update failed: %v", err)
		}

		found := false
		for _, r := range records {
			if r.Hostname == "inttest-a.dnsweaver.local" && r.Target == "10.99.99.2" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Updated record not found in List()")
		}

		// Update testRecords for cleanup
		testRecords[0].Target = "10.99.99.2"
	})

	// 5. TXT record silently skipped
	t.Run("TXT_Skipped", func(t *testing.T) {
		err := p.Create(ctx, provider.Record{
			Hostname: "inttest-txt.dnsweaver.local",
			Type:     provider.RecordTypeTXT,
			Target:   "should-be-skipped",
		})
		if err != nil {
			t.Errorf("Create(TXT) should silently skip, got error: %v", err)
		}
	})

	// 6. Delete all test records
	t.Run("Delete", func(t *testing.T) {
		for _, rec := range testRecords {
			if err := p.Delete(ctx, rec); err != nil {
				t.Errorf("Delete(%s %s) failed: %v", rec.Hostname, rec.Type, err)
			}
		}

		// Verify all deleted
		records, err := p.List(ctx)
		if err != nil {
			t.Fatalf("List() after delete failed: %v", err)
		}

		for _, r := range records {
			for _, expected := range testRecords {
				if r.Hostname == expected.Hostname {
					t.Errorf("Record %s should have been deleted", r.Hostname)
				}
			}
		}
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
