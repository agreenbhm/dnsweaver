package powerdns

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// newTestProvider builds a Provider pointed at the given base URL.
func newTestProvider(t *testing.T, baseURL string) *Provider {
	t.Helper()
	p, err := New("test-pdns", &Config{
		URL: baseURL, APIKey: "secret", Zone: "example.com", ServerID: "localhost", TTL: 300,
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}
	return p
}

func TestProvider_TypeAndCapabilities(t *testing.T) {
	p := newTestProvider(t, "http://ns1:8081")
	if p.Type() != "powerdns" {
		t.Errorf("Type() = %q", p.Type())
	}
	caps := p.Capabilities()
	if !caps.SupportsOwnershipTXT || !caps.SupportsNativeUpdate {
		t.Error("expected ownership TXT and native update support")
	}
	for _, rt := range []provider.RecordType{
		provider.RecordTypeA, provider.RecordTypeAAAA, provider.RecordTypeCNAME,
		provider.RecordTypeSRV, provider.RecordTypeTXT,
	} {
		if !caps.SupportsRecordType(rt) {
			t.Errorf("expected support for %s", rt)
		}
	}
}

func TestProvider_Identity(t *testing.T) {
	p := newTestProvider(t, "http://ns1:8081")
	id := p.Identity()
	if id.Type != "powerdns" || id.Endpoint != "http://ns1:8081" || id.Zone != "example.com" {
		t.Errorf("Identity = %+v", id)
	}
}

func TestProvider_Ping(t *testing.T) {
	// pingHandler routes the two calls provider.Ping makes: the server-list
	// connectivity check (GET .../servers) and the zone existence check.
	pingHandler := func(serversStatus, zoneStatus int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/servers") {
				if serversStatus != http.StatusOK {
					w.WriteHeader(serversStatus)
					return
				}
				w.Write([]byte(`[{"id":"localhost"}]`))
				return
			}
			if zoneStatus != http.StatusOK {
				w.WriteHeader(zoneStatus)
				return
			}
			json.NewEncoder(w).Encode(zoneResponse{Name: "example.com."})
		}
	}

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(pingHandler(http.StatusOK, http.StatusOK))
		defer srv.Close()
		if err := newTestProvider(t, srv.URL).Ping(context.Background()); err != nil {
			t.Errorf("Ping error: %v", err)
		}
	})
	t.Run("zone not found", func(t *testing.T) {
		srv := httptest.NewServer(pingHandler(http.StatusOK, http.StatusNotFound))
		defer srv.Close()
		err := newTestProvider(t, srv.URL).Ping(context.Background())
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected actionable zone-not-found error, got %v", err)
		}
	})
	t.Run("unauthorized", func(t *testing.T) {
		srv := httptest.NewServer(pingHandler(http.StatusUnauthorized, http.StatusOK))
		defer srv.Close()
		err := newTestProvider(t, srv.URL).Ping(context.Background())
		if !errors.Is(err, provider.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}

func TestProvider_List(t *testing.T) {
	zone := zoneResponse{
		Name: "example.com.",
		RRsets: []rrset{
			{Name: "app.example.com.", Type: "A", TTL: 300, Records: []apiRecord{
				{Content: "192.0.2.1"}, {Content: "192.0.2.2"},
			}},
			{Name: "cname.example.com.", Type: "CNAME", TTL: 300, Records: []apiRecord{{Content: "app.example.com."}}},
			{Name: "_dnsweaver.app.example.com.", Type: "TXT", TTL: 300, Records: []apiRecord{{Content: `"heritage=dnsweaver"`}}},
			{Name: "sip.example.com.", Type: "SRV", TTL: 300, Records: []apiRecord{{Content: "10 20 5060 host.example.com."}}},
			{Name: "example.com.", Type: "SOA", TTL: 3600, Records: []apiRecord{{Content: "ns1. hostmaster. 1 1 1 1 1"}}},
			{Name: "disabled.example.com.", Type: "A", TTL: 300, Records: []apiRecord{{Content: "192.0.2.9", Disabled: true}}},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(zone)
	}))
	defer srv.Close()

	records, err := newTestProvider(t, srv.URL).List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	// 2 A + 1 CNAME + 1 TXT + 1 SRV = 5 (SOA filtered out, disabled skipped)
	if len(records) != 5 {
		t.Fatalf("got %d records, want 5: %+v", len(records), records)
	}
	byKey := map[string]provider.Record{}
	for _, r := range records {
		byKey[string(r.Type)+"|"+r.Hostname+"|"+r.Target] = r
	}
	if _, ok := byKey["CNAME|cname.example.com|app.example.com"]; !ok {
		t.Error("CNAME target should be dot-stripped")
	}
	if _, ok := byKey["TXT|_dnsweaver.app.example.com|heritage=dnsweaver"]; !ok {
		t.Error("TXT content should be unquoted")
	}
	srvRec, ok := byKey["SRV|sip.example.com|host.example.com"]
	if !ok || srvRec.SRV == nil || srvRec.SRV.Port != 5060 {
		t.Errorf("SRV record not parsed correctly: %+v", srvRec)
	}
}

func TestProvider_List_SkipsUndecodableRecord(t *testing.T) {
	zone := zoneResponse{
		Name: "example.com.",
		RRsets: []rrset{
			{Name: "bad.example.com.", Type: "SRV", TTL: 300, Records: []apiRecord{
				{Content: "this is not valid srv"},
			}},
			{Name: "good.example.com.", Type: "A", TTL: 300, Records: []apiRecord{
				{Content: "192.0.2.1"},
			}},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(zone)
	}))
	defer srv.Close()

	records, err := newTestProvider(t, srv.URL).List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	// Should skip the malformed SRV, include only the valid A record
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1: %+v", len(records), records)
	}
	if records[0].Hostname != "good.example.com" || records[0].Target != "192.0.2.1" {
		t.Errorf("expected valid A record (good.example.com/192.0.2.1), got %+v", records[0])
	}
}

// mockPDNS is an in-memory PowerDNS zone that serves GET and records PATCHes.
type mockPDNS struct {
	zone    zoneResponse
	patches []patchRequest
}

func (m *mockPDNS) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(m.zone)
		case http.MethodPatch:
			var pr patchRequest
			if err := json.NewDecoder(r.Body).Decode(&pr); err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			m.patches = append(m.patches, pr)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
}

func (m *mockPDNS) lastRRset(t *testing.T) rrset {
	t.Helper()
	if len(m.patches) == 0 {
		t.Fatal("no PATCH recorded")
	}
	last := m.patches[len(m.patches)-1]
	if len(last.RRsets) != 1 {
		t.Fatalf("expected 1 rrset in patch, got %d", len(last.RRsets))
	}
	return last.RRsets[0]
}

func TestProvider_Create_EmptyZone(t *testing.T) {
	m := &mockPDNS{zone: zoneResponse{Name: "example.com."}}
	srv := m.server(t)
	defer srv.Close()

	err := newTestProvider(t, srv.URL).Create(context.Background(), provider.Record{
		Hostname: "app.example.com", Type: provider.RecordTypeA, Target: "192.0.2.1", TTL: 120,
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	rs := m.lastRRset(t)
	if rs.Name != "app.example.com." || rs.Type != "A" || rs.ChangeType != "REPLACE" {
		t.Errorf("unexpected rrset: %+v", rs)
	}
	if rs.TTL != 120 {
		t.Errorf("TTL = %d, want 120", rs.TTL)
	}
	if len(rs.Records) != 1 || rs.Records[0].Content != "192.0.2.1" {
		t.Errorf("records = %+v", rs.Records)
	}
}

func TestProvider_Create_PreservesSiblings(t *testing.T) {
	m := &mockPDNS{zone: zoneResponse{Name: "example.com.", RRsets: []rrset{
		{Name: "app.example.com.", Type: "A", TTL: 300, Records: []apiRecord{{Content: "192.0.2.1"}}},
	}}}
	srv := m.server(t)
	defer srv.Close()

	err := newTestProvider(t, srv.URL).Create(context.Background(), provider.Record{
		Hostname: "app.example.com", Type: provider.RecordTypeA, Target: "192.0.2.2",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	rs := m.lastRRset(t)
	if len(rs.Records) != 2 {
		t.Fatalf("expected 2 records (round-robin), got %+v", rs.Records)
	}
	contents := map[string]bool{}
	for _, r := range rs.Records {
		contents[r.Content] = true
	}
	if !contents["192.0.2.1"] || !contents["192.0.2.2"] {
		t.Errorf("expected both IPs in merged set, got %+v", rs.Records)
	}
}

func TestProvider_Create_Idempotent(t *testing.T) {
	m := &mockPDNS{zone: zoneResponse{Name: "example.com.", RRsets: []rrset{
		{Name: "app.example.com.", Type: "A", TTL: 300, Records: []apiRecord{{Content: "192.0.2.1"}}},
	}}}
	srv := m.server(t)
	defer srv.Close()

	err := newTestProvider(t, srv.URL).Create(context.Background(), provider.Record{
		Hostname: "app.example.com", Type: provider.RecordTypeA, Target: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if len(m.patches) != 0 {
		t.Errorf("expected no PATCH for idempotent create, got %d", len(m.patches))
	}
}

func TestProvider_Create_TXTQuoted(t *testing.T) {
	m := &mockPDNS{zone: zoneResponse{Name: "example.com."}}
	srv := m.server(t)
	defer srv.Close()

	err := newTestProvider(t, srv.URL).Create(context.Background(), provider.Record{
		Hostname: "_dnsweaver.app.example.com", Type: provider.RecordTypeTXT, Target: "heritage=dnsweaver",
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	rs := m.lastRRset(t)
	if rs.Records[0].Content != `"heritage=dnsweaver"` {
		t.Errorf("TXT content = %q, want quoted", rs.Records[0].Content)
	}
}

func TestProvider_Create_TTLFallback(t *testing.T) {
	m := &mockPDNS{zone: zoneResponse{Name: "example.com."}}
	srv := m.server(t)
	defer srv.Close()

	err := newTestProvider(t, srv.URL).Create(context.Background(), provider.Record{
		Hostname: "app.example.com", Type: provider.RecordTypeA, Target: "10.0.0.1",
		// TTL deliberately omitted (0) -> provider default (300) must be used
	})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	rs := m.lastRRset(t)
	if rs.TTL != 300 {
		t.Errorf("TTL = %d, want 300 (provider default)", rs.TTL)
	}
}

func TestProvider_Delete_LastRecord(t *testing.T) {
	m := &mockPDNS{zone: zoneResponse{Name: "example.com.", RRsets: []rrset{
		{Name: "app.example.com.", Type: "A", TTL: 300, Records: []apiRecord{{Content: "192.0.2.1"}}},
	}}}
	srv := m.server(t)
	defer srv.Close()

	err := newTestProvider(t, srv.URL).Delete(context.Background(), provider.Record{
		Hostname: "app.example.com", Type: provider.RecordTypeA, Target: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	rs := m.lastRRset(t)
	if rs.ChangeType != "DELETE" {
		t.Errorf("changetype = %q, want DELETE when rrset emptied", rs.ChangeType)
	}
}

func TestProvider_Delete_KeepsSiblings(t *testing.T) {
	m := &mockPDNS{zone: zoneResponse{Name: "example.com.", RRsets: []rrset{
		{Name: "app.example.com.", Type: "A", TTL: 300, Records: []apiRecord{
			{Content: "192.0.2.1"}, {Content: "192.0.2.2"},
		}},
	}}}
	srv := m.server(t)
	defer srv.Close()

	err := newTestProvider(t, srv.URL).Delete(context.Background(), provider.Record{
		Hostname: "app.example.com", Type: provider.RecordTypeA, Target: "192.0.2.1",
	})
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	rs := m.lastRRset(t)
	if rs.ChangeType != "REPLACE" || len(rs.Records) != 1 || rs.Records[0].Content != "192.0.2.2" {
		t.Errorf("expected REPLACE keeping 192.0.2.2, got %+v", rs)
	}
}

func TestProvider_Delete_NoOps(t *testing.T) {
	// rrset absent entirely
	m1 := &mockPDNS{zone: zoneResponse{Name: "example.com."}}
	s1 := m1.server(t)
	defer s1.Close()
	if err := newTestProvider(t, s1.URL).Delete(context.Background(), provider.Record{
		Hostname: "gone.example.com", Type: provider.RecordTypeA, Target: "192.0.2.9",
	}); err != nil {
		t.Fatalf("Delete (absent rrset) error: %v", err)
	}
	if len(m1.patches) != 0 {
		t.Errorf("expected no PATCH when rrset absent, got %d", len(m1.patches))
	}

	// rrset present but target not in it
	m2 := &mockPDNS{zone: zoneResponse{Name: "example.com.", RRsets: []rrset{
		{Name: "app.example.com.", Type: "A", TTL: 300, Records: []apiRecord{{Content: "192.0.2.1"}}},
	}}}
	s2 := m2.server(t)
	defer s2.Close()
	if err := newTestProvider(t, s2.URL).Delete(context.Background(), provider.Record{
		Hostname: "app.example.com", Type: provider.RecordTypeA, Target: "192.0.2.99",
	}); err != nil {
		t.Fatalf("Delete (absent content) error: %v", err)
	}
	if len(m2.patches) != 0 {
		t.Errorf("expected no PATCH when content absent, got %d", len(m2.patches))
	}
}
