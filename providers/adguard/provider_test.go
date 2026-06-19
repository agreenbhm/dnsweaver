package adguard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/maxfield-allison/dnsweaver/pkg/provider"
)

func TestProvider_Name(t *testing.T) {
	p := &Provider{name: "my-adguard"}
	if got := p.Name(); got != "my-adguard" {
		t.Errorf("Name() = %q, want %q", got, "my-adguard")
	}
}

func TestProvider_Type(t *testing.T) {
	p := &Provider{}
	if got := p.Type(); got != "adguard" {
		t.Errorf("Type() = %q, want %q", got, "adguard")
	}
}

func TestProvider_Capabilities(t *testing.T) {
	p := &Provider{}
	caps := p.Capabilities()

	if caps.SupportsOwnershipTXT {
		t.Error("SupportsOwnershipTXT should be false")
	}
	if !caps.SupportsNativeUpdate {
		t.Error("SupportsNativeUpdate should be true")
	}

	supported := map[provider.RecordType]bool{
		provider.RecordTypeA:     false,
		provider.RecordTypeAAAA:  false,
		provider.RecordTypeCNAME: false,
	}
	for _, rt := range caps.SupportedRecordTypes {
		supported[rt] = true
	}
	for rt, found := range supported {
		if !found {
			t.Errorf("missing supported record type: %s", rt)
		}
	}
}

func TestProvider_List(t *testing.T) {
	enabled := true
	disabled := false

	tests := []struct {
		name      string
		zone      string
		entries   []rewriteEntry
		wantCount int
		wantErr   bool
	}{
		{
			name: "all records no zone filter",
			zone: "",
			entries: []rewriteEntry{
				{Domain: "server.home.local", Answer: "192.168.1.100", Enabled: &enabled},
				{Domain: "nas.home.local", Answer: "2001:db8::1", Enabled: &enabled},
				{Domain: "alias.home.local", Answer: "server.home.local", Enabled: &enabled},
			},
			wantCount: 3,
		},
		{
			name: "zone filtering",
			zone: "home.local",
			entries: []rewriteEntry{
				{Domain: "server.home.local", Answer: "192.168.1.100", Enabled: &enabled},
				{Domain: "other.example.com", Answer: "10.0.0.1", Enabled: &enabled},
			},
			wantCount: 1,
		},
		{
			name: "skip disabled entries",
			zone: "",
			entries: []rewriteEntry{
				{Domain: "active.local", Answer: "1.2.3.4", Enabled: &enabled},
				{Domain: "disabled.local", Answer: "5.6.7.8", Enabled: &disabled},
			},
			wantCount: 1,
		},
		{
			name: "skip empty answer entries",
			zone: "",
			entries: []rewriteEntry{
				{Domain: "server.local", Answer: "1.2.3.4", Enabled: &enabled},
				{Domain: "blocked.local", Answer: "", Enabled: &enabled},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(tt.entries)
			}))
			defer server.Close()

			cfg := &Config{
				URL:      server.URL,
				Username: "admin",
				Password: "secret",
				Zone:     tt.zone,
				TTL:      300,
			}

			p, err := New("test", cfg, WithProviderHTTPClient(server.Client()))
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			records, err := p.List(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(records) != tt.wantCount {
				t.Errorf("List() returned %d records, want %d", len(records), tt.wantCount)
			}
		})
	}
}

func TestProvider_List_RecordTypes(t *testing.T) {
	enabled := true

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries := []rewriteEntry{
			{Domain: "ipv4.local", Answer: "192.168.1.1", Enabled: &enabled},
			{Domain: "ipv6.local", Answer: "2001:db8::1", Enabled: &enabled},
			{Domain: "cname.local", Answer: "target.example.com", Enabled: &enabled},
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(entries)
	}))
	defer server.Close()

	cfg := &Config{
		URL:      server.URL,
		Username: "admin",
		Password: "secret",
		TTL:      300,
	}

	p, err := New("test", cfg, WithProviderHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	records, err := p.List(context.Background())
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("List() returned %d records, want 3", len(records))
	}

	hasRecord := func(hostname string, recordType provider.RecordType, target string) bool {
		for _, r := range records {
			if r.Hostname == hostname && r.Type == recordType && r.Target == target {
				return true
			}
		}
		return false
	}

	if !hasRecord("ipv4.local", provider.RecordTypeA, "192.168.1.1") {
		t.Error("missing A record: ipv4.local -> 192.168.1.1")
	}
	if !hasRecord("ipv6.local", provider.RecordTypeAAAA, "2001:db8::1") {
		t.Error("missing AAAA record: ipv6.local -> 2001:db8::1")
	}
	if !hasRecord("cname.local", provider.RecordTypeCNAME, "target.example.com") {
		t.Error("missing CNAME record: cname.local -> target.example.com")
	}
}

func TestProvider_Create(t *testing.T) {
	tests := []struct {
		name       string
		record     provider.Record
		statusCode int
		wantErr    bool
		wantSkip   bool // expect silent skip (no API call)
	}{
		{
			name: "A record",
			record: provider.Record{
				Hostname: "new.local",
				Type:     provider.RecordTypeA,
				Target:   "10.0.0.1",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "AAAA record",
			record: provider.Record{
				Hostname: "new.local",
				Type:     provider.RecordTypeAAAA,
				Target:   "2001:db8::1",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "CNAME record",
			record: provider.Record{
				Hostname: "alias.local",
				Type:     provider.RecordTypeCNAME,
				Target:   "target.local",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "TXT record silently skipped",
			record: provider.Record{
				Hostname: "txt.local",
				Type:     provider.RecordTypeTXT,
				Target:   "some-text",
			},
			wantErr:  false,
			wantSkip: true,
		},
		{
			name: "unsupported SRV record",
			record: provider.Record{
				Hostname: "srv.local",
				Type:     provider.RecordTypeSRV,
				Target:   "target.local",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			cfg := &Config{
				URL:      server.URL,
				Username: "admin",
				Password: "secret",
				TTL:      300,
			}

			p, err := New("test", cfg, WithProviderHTTPClient(server.Client()))
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			err = p.Create(context.Background(), tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantSkip && called {
				t.Error("Create() made API call for unsupported record type (expected skip)")
			}
		})
	}
}

func TestProvider_Delete(t *testing.T) {
	tests := []struct {
		name     string
		record   provider.Record
		wantErr  bool
		wantSkip bool
	}{
		{
			name: "A record",
			record: provider.Record{
				Hostname: "old.local",
				Type:     provider.RecordTypeA,
				Target:   "10.0.0.1",
			},
			wantErr: false,
		},
		{
			name: "TXT record silently skipped",
			record: provider.Record{
				Hostname: "txt.local",
				Type:     provider.RecordTypeTXT,
				Target:   "some-text",
			},
			wantErr:  false,
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cfg := &Config{
				URL:      server.URL,
				Username: "admin",
				Password: "secret",
				TTL:      300,
			}

			p, err := New("test", cfg, WithProviderHTTPClient(server.Client()))
			if err != nil {
				t.Fatalf("New() error: %v", err)
			}

			err = p.Delete(context.Background(), tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantSkip && called {
				t.Error("Delete() made API call for unsupported record type (expected skip)")
			}
		})
	}
}

func TestProvider_Update(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/rewrite/update" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var received rewriteUpdate
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode error: %v", err)
		}

		if received.Target.Domain != "server.local" || received.Target.Answer != "10.0.0.1" {
			t.Errorf("wrong target: %+v", received.Target)
		}
		if received.Update.Domain != "server.local" || received.Update.Answer != "10.0.0.2" {
			t.Errorf("wrong update: %+v", received.Update)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		URL:      server.URL,
		Username: "admin",
		Password: "secret",
		TTL:      300,
	}

	p, err := New("test", cfg, WithProviderHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	existing := provider.Record{Hostname: "server.local", Type: provider.RecordTypeA, Target: "10.0.0.1"}
	desired := provider.Record{Hostname: "server.local", Type: provider.RecordTypeA, Target: "10.0.0.2"}

	err = p.Update(context.Background(), existing, desired)
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
}

func TestClassifyAnswer(t *testing.T) {
	tests := []struct {
		answer string
		want   provider.RecordType
	}{
		{"192.168.1.1", provider.RecordTypeA},
		{"10.0.0.1", provider.RecordTypeA},
		{"2001:db8::1", provider.RecordTypeAAAA},
		{"::1", provider.RecordTypeAAAA},
		{"server.local", provider.RecordTypeCNAME},
		{"example.com", provider.RecordTypeCNAME},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.answer, func(t *testing.T) {
			got := classifyAnswer(tt.answer)
			if got != tt.want {
				t.Errorf("classifyAnswer(%q) = %q, want %q", tt.answer, got, tt.want)
			}
		})
	}
}

func TestNew_NilConfig(t *testing.T) {
	_, err := New("test", nil)
	if err == nil {
		t.Error("New() with nil config should return error")
	}
}

func TestNew_InvalidConfig(t *testing.T) {
	_, err := New("test", &Config{})
	if err == nil {
		t.Error("New() with empty config should return error")
	}
}
