package provider

import (
	"crypto/tls"
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/httputil"
)

// TestRegistry_PropagatesTLSConfig is a regression test for the TLS hardening
// work tracked in #183 / GitHub #89. It pins the contract that every TLS_*
// key written into ProviderConfig by the loader is materialized into
// FactoryConfig.HTTP.TLS before the factory runs, so individual providers do
// NOT need to know anything about TLS configuration themselves.
func TestRegistry_PropagatesTLSConfig(t *testing.T) {
	r := NewRegistry(testLogger())

	var captured FactoryConfig
	r.RegisterFactory("capture", func(cfg FactoryConfig) (Provider, error) {
		captured = cfg
		return &mockProvider{name: cfg.Name, typeName: "capture"}, nil
	})

	err := r.CreateInstance(ProviderInstanceConfig{
		Name:       "tls-instance",
		TypeName:   "capture",
		RecordType: RecordTypeA,
		Target:     "10.0.0.1",
		TTL:        300,
		Domains:    []string{"*.example.com"},
		ProviderConfig: map[string]string{
			"URL":             "https://dns.example.com",
			"TOKEN":           "abc",
			"ZONE":            "example.com",
			"TLS_CA_FILE":     "/etc/ssl/internal-ca.pem",
			"TLS_CERT_FILE":   "/etc/ssl/client.crt",
			"TLS_KEY_FILE":    "/etc/ssl/client.key",
			"TLS_SERVER_NAME": "dns.internal",
			"TLS_SKIP_VERIFY": "true",
			"TLS_MIN_VERSION": "1.3",
		},
	})
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}

	if captured.HTTP.TLS == nil {
		t.Fatal("HTTP.TLS not propagated to factory")
	}
	got := captured.HTTP.TLS
	want := &httputil.TLSConfig{
		CAFile:       "/etc/ssl/internal-ca.pem",
		CertFile:     "/etc/ssl/client.crt",
		KeyFile:      "/etc/ssl/client.key",
		ServerName:   "dns.internal",
		InsecureSkip: true,
		MinVersion:   tls.VersionTLS13,
	}
	if *got != *want {
		t.Errorf("TLS config mismatch\n got: %+v\nwant: %+v", *got, *want)
	}
}

// TestRegistry_NoTLSKeys_PropagatesNil ensures we don't synthesize an empty
// TLSConfig when the operator hasn't asked for one — the factory should see
// HTTP.TLS == nil so the default transport applies.
func TestRegistry_NoTLSKeys_PropagatesNil(t *testing.T) {
	r := NewRegistry(testLogger())

	var captured FactoryConfig
	r.RegisterFactory("capture", func(cfg FactoryConfig) (Provider, error) {
		captured = cfg
		return &mockProvider{name: cfg.Name, typeName: "capture"}, nil
	})

	err := r.CreateInstance(ProviderInstanceConfig{
		Name:       "default-tls",
		TypeName:   "capture",
		RecordType: RecordTypeA,
		Target:     "10.0.0.1",
		TTL:        300,
		Domains:    []string{"*.example.com"},
		ProviderConfig: map[string]string{
			"URL":   "https://dns.example.com",
			"TOKEN": "abc",
			"ZONE":  "example.com",
		},
	})
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if captured.HTTP.TLS != nil {
		t.Errorf("expected HTTP.TLS to be nil, got %+v", *captured.HTTP.TLS)
	}
}
