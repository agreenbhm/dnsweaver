package testutil

import (
	"strings"
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// RequireNoError fails the test immediately if err is not nil.
func RequireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// RequireError fails the test immediately if err is nil.
func RequireError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}

// RequireErrorContains fails if err is nil or doesn't contain substr.
func RequireErrorContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q but got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("expected error containing %q, got: %v", substr, err)
	}
}

// AssertEqual fails the test if got != want.
func AssertEqual[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", field, got, want)
	}
}

// AssertContains fails the test if s does not contain substr.
func AssertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

// AssertNotContains fails the test if s contains substr.
func AssertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected %q to NOT contain %q", s, substr)
	}
}

// AssertLen fails the test if the slice length doesn't match.
func AssertLen[T any](t *testing.T, name string, slice []T, expected int) {
	t.Helper()
	if len(slice) != expected {
		t.Errorf("%s: expected %d items, got %d", name, expected, len(slice))
	}
}

// --- Record-specific assertions ---

// AssertRecordExists fails if no record matches hostname and record type.
func AssertRecordExists(t *testing.T, records []provider.Record, hostname string, recordType provider.RecordType) {
	t.Helper()
	for _, r := range records {
		if r.Hostname == hostname && r.Type == recordType {
			return
		}
	}
	t.Errorf("expected record %s/%s not found in %d records", hostname, recordType, len(records))
}

// AssertRecordNotExists fails if any record matches hostname and record type.
func AssertRecordNotExists(t *testing.T, records []provider.Record, hostname string, recordType provider.RecordType) {
	t.Helper()
	for _, r := range records {
		if r.Hostname == hostname && r.Type == recordType {
			t.Errorf("unexpected record %s/%s found in records", hostname, recordType)
			return
		}
	}
}

// FindRecord returns the first record matching hostname and type, or nil.
func FindRecord(records []provider.Record, hostname string, recordType provider.RecordType) *provider.Record {
	for i := range records {
		if records[i].Hostname == hostname && records[i].Type == recordType {
			return &records[i]
		}
	}
	return nil
}

// FindRecordByTarget returns the first record matching hostname, type, and target, or nil.
func FindRecordByTarget(records []provider.Record, hostname string, recordType provider.RecordType, target string) *provider.Record {
	for i := range records {
		if records[i].Hostname == hostname && records[i].Type == recordType && records[i].Target == target {
			return &records[i]
		}
	}
	return nil
}

// AssertRecordTarget fails if no record matches hostname/type/target.
func AssertRecordTarget(t *testing.T, records []provider.Record, hostname string, recordType provider.RecordType, target string) {
	t.Helper()
	r := FindRecordByTarget(records, hostname, recordType, target)
	if r == nil {
		t.Errorf("expected record %s/%s -> %s not found in %d records", hostname, recordType, target, len(records))
	}
}
