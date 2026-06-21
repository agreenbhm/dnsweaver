package pihole

import (
	"github.com/maxfield-allison/dnsweaver/pkg/httputil"
	"github.com/maxfield-allison/dnsweaver/pkg/provider"
)

// Factory returns a provider.Factory for creating Pi-hole provider instances.
// This is the recommended way to register the Pi-hole provider with the registry.
func Factory() provider.Factory {
	return func(cfg provider.FactoryConfig) (provider.Provider, error) {
		// Parse provider-specific configuration from the map
		providerCfg, err := LoadConfigFromMap(cfg.Name, cfg.ProviderConfig)
		if err != nil {
			return nil, err
		}

		// Build provider options
		opts := []ProviderOption{
			WithProviderLogger(cfg.HTTP.Logger),
		}

		// Only create HTTP client for API mode. File mode never touches the
		// network, so no TLS configuration applies.
		if providerCfg.Mode == ModeAPI {
			httpClient := httputil.NewClient(&httputil.ClientConfig{
				Timeout:   cfg.HTTP.Timeout,
				TLS:       cfg.HTTP.TLS,
				UserAgent: cfg.HTTP.UserAgent,
				Logger:    cfg.HTTP.Logger,
			})
			opts = append(opts, WithProviderHTTPClient(httpClient))
		}

		// Create the provider
		return New(cfg.Name, providerCfg, opts...)
	}
}
