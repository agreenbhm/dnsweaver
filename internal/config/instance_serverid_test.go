package config

import "testing"

func TestProviderConfigFields_IncludesServerID(t *testing.T) {
	for _, f := range providerConfigFields {
		if f.name == "SERVER_ID" {
			return
		}
	}
	t.Error("SERVER_ID must be in providerConfigFields so PowerDNS env config is honored")
}
