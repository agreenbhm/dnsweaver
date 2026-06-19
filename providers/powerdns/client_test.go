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

func TestCanonicalizeAndStrip(t *testing.T) {
	if got := canonicalize("app.example.com"); got != "app.example.com." {
		t.Errorf("canonicalize = %q", got)
	}
	if got := canonicalize("app.example.com."); got != "app.example.com." {
		t.Errorf("canonicalize idempotent failed: %q", got)
	}
	if got := stripDot("app.example.com."); got != "app.example.com" {
		t.Errorf("stripDot = %q", got)
	}
	if got := stripDot("app.example.com"); got != "app.example.com" {
		t.Errorf("stripDot no-dot failed: %q", got)
	}
}

func TestTXTQuotingRoundTrip(t *testing.T) {
	cases := []string{
		"heritage=dnsweaver",
		"heritage=dnsweaver,instance=pi5",
		`value with "quotes" inside`,
		// A value whose content genuinely begins and ends with a double-quote;
		// the old idempotency guard would have silently corrupted this.
		`"quoted"`,
		// A value containing a backslash.
		`back\slash`,
	}
	for _, in := range cases {
		q := quoteTXT(in)
		if q[0] != '"' || q[len(q)-1] != '"' {
			t.Errorf("quoteTXT(%q) not wrapped: %q", in, q)
		}
		if got := unquoteTXT(q); got != in {
			t.Errorf("round-trip failed: in=%q quoted=%q out=%q", in, q, got)
		}
	}
}

func TestSRVEncodeParse(t *testing.T) {
	srv := &provider.SRVData{Priority: 10, Weight: 20, Port: 5060}
	content := encodeSRVContent(srv, "sip.example.com")
	if content != "10 20 5060 sip.example.com." {
		t.Errorf("encodeSRVContent = %q", content)
	}
	got, target, err := parseSRVContent(content)
	if err != nil {
		t.Fatalf("parseSRVContent error: %v", err)
	}
	if target != "sip.example.com" {
		t.Errorf("parsed target = %q", target)
	}
	if *got != *srv {
		t.Errorf("parsed SRV = %+v, want %+v", got, srv)
	}
	if _, _, err := parseSRVContent("10 20 sip.example.com."); err == nil {
		t.Error("expected error for malformed SRV content")
	}
	// Non-numeric priority field must return an error.
	if _, _, err := parseSRVContent("a 20 5060 host.example.com."); err == nil {
		t.Error("expected error for non-numeric SRV priority")
	}
	// Non-numeric weight field must return an error.
	if _, _, err := parseSRVContent("10 b 5060 host.example.com."); err == nil {
		t.Error("expected error for non-numeric SRV weight")
	}
	// Non-numeric port field must return an error.
	if _, _, err := parseSRVContent("10 20 c host.example.com."); err == nil {
		t.Error("expected error for non-numeric SRV port")
	}
}

func TestRecordContentEncode(t *testing.T) {
	cases := []struct {
		rec  provider.Record
		want string
	}{
		{provider.Record{Type: provider.RecordTypeA, Target: "192.0.2.1"}, "192.0.2.1"},
		{provider.Record{Type: provider.RecordTypeAAAA, Target: "2001:db8::1"}, "2001:db8::1"},
		{provider.Record{Type: provider.RecordTypeCNAME, Target: "host.example.com"}, "host.example.com."},
		{provider.Record{Type: provider.RecordTypeTXT, Target: "heritage=dnsweaver"}, `"heritage=dnsweaver"`},
		{provider.Record{Type: provider.RecordTypeSRV, Target: "sip.example.com", SRV: &provider.SRVData{Priority: 1, Weight: 2, Port: 3}}, "1 2 3 sip.example.com."},
	}
	for _, tt := range cases {
		got, err := recordContent(tt.rec)
		if err != nil {
			t.Errorf("recordContent(%v) error: %v", tt.rec.Type, err)
			continue
		}
		if got != tt.want {
			t.Errorf("recordContent(%v) = %q, want %q", tt.rec.Type, got, tt.want)
		}
	}
	if _, err := recordContent(provider.Record{Type: provider.RecordTypeSRV}); err == nil {
		t.Error("expected error for SRV record without SRV data")
	}
}

func TestDecodeContent(t *testing.T) {
	// Identity case: A record passes through unchanged.
	target, _, err := decodeContent(provider.RecordTypeA, "192.0.2.1")
	if err != nil {
		t.Fatalf("A decode error: %v", err)
	}
	if target != "192.0.2.1" {
		t.Errorf("A decode = %q, want %q", target, "192.0.2.1")
	}

	target, _, err = decodeContent(provider.RecordTypeCNAME, "host.example.com.")
	if err != nil {
		t.Fatalf("CNAME decode error: %v", err)
	}
	if target != "host.example.com" {
		t.Errorf("CNAME decode = %q", target)
	}

	target, _, err = decodeContent(provider.RecordTypeTXT, `"heritage=dnsweaver"`)
	if err != nil {
		t.Fatalf("TXT decode error: %v", err)
	}
	if target != "heritage=dnsweaver" {
		t.Errorf("TXT decode = %q", target)
	}

	target, srv, err := decodeContent(provider.RecordTypeSRV, "1 2 3 sip.example.com.")
	if err != nil || srv == nil || target != "sip.example.com" || srv.Port != 3 {
		t.Errorf("SRV decode = %q %+v err=%v", target, srv, err)
	}
}

func TestClient_GetZone_SetsAPIKeyAndPath(t *testing.T) {
	var gotKey, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(zoneResponse{Name: "example.com.", RRsets: []rrset{
			{Name: "app.example.com.", Type: "A", TTL: 300, Records: []apiRecord{{Content: "192.0.2.1"}}},
		}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret-key", "localhost")
	z, err := c.GetZone(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("GetZone error: %v", err)
	}
	if gotKey != "secret-key" {
		t.Errorf("X-API-Key = %q, want secret-key", gotKey)
	}
	if gotPath != "/api/v1/servers/localhost/zones/example.com." {
		t.Errorf("path = %q", gotPath)
	}
	if len(z.RRsets) != 1 || z.RRsets[0].Records[0].Content != "192.0.2.1" {
		t.Errorf("unexpected zone payload: %+v", z)
	}
}

func TestClient_GetZone_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(apiErrorBody{Error: "Not Found"})
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "k", "localhost")
	_, err := c.GetZone(context.Background(), "example.com")
	if !errors.Is(err, errZoneNotFound) {
		t.Errorf("expected errZoneNotFound, got %v", err)
	}
}

func TestClient_ErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"unauthorized", http.StatusUnauthorized, provider.ErrUnauthorized},
		{"forbidden", http.StatusForbidden, provider.ErrUnauthorized},
		{"server error", http.StatusInternalServerError, provider.ErrProviderUnavailable},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				json.NewEncoder(w).Encode(apiErrorBody{Error: "boom"})
			}))
			defer srv.Close()
			c := NewClient(srv.URL, "k", "localhost")
			_, err := c.GetZone(context.Background(), "example.com")
			if !errors.Is(err, tt.want) {
				t.Errorf("status %d: got %v, want wrapping %v", tt.status, err, tt.want)
			}
		})
	}
}

func TestClient_Unprocessable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(apiErrorBody{Error: "Conflicting record"})
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "k", "localhost")
	err := c.PatchRRsets(context.Background(), "example.com", []rrset{{Name: "x.example.com.", Type: "A"}})
	if err == nil || !strings.Contains(err.Error(), "Conflicting record") {
		t.Errorf("expected 422 error carrying server message, got %v", err)
	}
}

func TestClient_PatchRRsets_SendsBody(t *testing.T) {
	var got patchRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.Header.Get("X-API-Key") != "k" {
			t.Errorf("X-API-Key = %q, want k", r.Header.Get("X-API-Key"))
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "k", "localhost")
	in := []rrset{{Name: "app.example.com.", Type: "A", TTL: 300, ChangeType: "REPLACE", Records: []apiRecord{{Content: "192.0.2.1"}}}}
	if err := c.PatchRRsets(context.Background(), "example.com", in); err != nil {
		t.Fatalf("PatchRRsets error: %v", err)
	}
	if len(got.RRsets) != 1 || got.RRsets[0].ChangeType != "REPLACE" || got.RRsets[0].Records[0].Content != "192.0.2.1" {
		t.Errorf("unexpected PATCH body: %+v", got)
	}
}

func TestClient_Ping(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Write([]byte(`[{"id":"localhost"}]`))
		}))
		defer srv.Close()
		if err := NewClient(srv.URL, "k", "localhost").Ping(context.Background()); err != nil {
			t.Fatalf("Ping error: %v", err)
		}
		if gotPath != "/api/v1/servers" {
			t.Errorf("ping path = %q, want /api/v1/servers", gotPath)
		}
	})
	t.Run("unauthorized", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		if err := NewClient(srv.URL, "k", "localhost").Ping(context.Background()); !errors.Is(err, provider.ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}
