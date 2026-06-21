package ovh

import (
	"github.com/maxfield-allison/dnsweaver/pkg/httputil"
	"github.com/maxfield-allison/dnsweaver/pkg/provider"
)

// Factory returns a provider.Factory for creating OVH provider instances.
// This is the recommended way to register the OVH provider with the registry.
func Factory() provider.Factory {
	return func(cfg provider.FactoryConfig) (provider.Provider, error) {
		// Parse provider-specific configuration from the map.
		providerCfg, err := LoadConfigFromMap(cfg.Name, cfg.ProviderConfig)
		if err != nil {
			return nil, err
		}

		// TLS settings are framework-wide via cfg.HTTP.TLS; httputil emits
		// its own WARN when verification is skipped.
		httpClient := httputil.NewClient(&httputil.ClientConfig{
			Timeout:   cfg.HTTP.Timeout,
			TLS:       cfg.HTTP.TLS,
			UserAgent: cfg.HTTP.UserAgent,
			Logger:    cfg.HTTP.Logger,
		})

		return New(cfg.Name, providerCfg,
			WithProviderHTTPClient(httpClient),
			WithProviderLogger(cfg.HTTP.Logger),
		)
	}
}
