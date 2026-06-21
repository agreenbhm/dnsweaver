package technitium

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/maxfield-allison/dnsweaver/pkg/provider"
)

func TestCompanionLifecycle(t *testing.T) {
	// Mock server to validate add/delete companion calls
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/zones/records/get":
			// Simulate no existing HTTPS records
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","response":{"zone":{"name":"zone"},"name":"app.example.com","records":[]}}`))
		case "/api/zones/records/add":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/zones/records/delete":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	cfg := &Config{URL: server.URL, Token: "token", Zone: "zone", TTL: 300, AutoHTTPSRecords: true, AutoHTTPSALPN: "h2"}
	p, err := New("inst", cfg)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	p.client = client

	ctx := context.Background()
	// create companion
	if err := p.createCompanionHTTPS(ctx, "app.example.com", provider.RecordTypeA, 300); err != nil {
		t.Fatalf("createCompanionHTTPS failed: %v", err)
	}
	// delete companion
	if err := p.deleteCompanionHTTPS(ctx, "app.example.com", provider.RecordTypeA); err != nil {
		t.Fatalf("deleteCompanionHTTPS failed: %v", err)
	}
}
