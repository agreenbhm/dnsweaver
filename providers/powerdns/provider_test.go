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
