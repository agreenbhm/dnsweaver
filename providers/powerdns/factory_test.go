package powerdns

import (
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

func TestFactory_CreatesProvider(t *testing.T) {
	p, err := Factory()(provider.FactoryConfig{
		Name: "my-pdns",
		ProviderConfig: map[string]string{
			"URL": "http://ns1:8081", "API_KEY": "secret", "ZONE": "example.com",
		},
	})
	if err != nil {
		t.Fatalf("Factory error: %v", err)
	}
	if p.Type() != "powerdns" {
		t.Errorf("Type() = %q", p.Type())
	}
	if p.Name() != "my-pdns" {
		t.Errorf("Name() = %q", p.Name())
	}
}

func TestFactory_MissingConfig(t *testing.T) {
	_, err := Factory()(provider.FactoryConfig{
		Name:           "my-pdns",
		ProviderConfig: map[string]string{"URL": "http://ns1:8081"},
	})
	if err == nil {
		t.Error("expected error for missing API_KEY/ZONE, got nil")
	}
}
