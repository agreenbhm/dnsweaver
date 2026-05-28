package webhook

import (
	"gitlab.bluewillows.net/root/dnsweaver/pkg/httputil"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// Factory returns a provider.Factory for creating Webhook provider instances.
// This is the recommended way to register the Webhook provider with the registry.
func Factory() provider.Factory {
	return func(cfg provider.FactoryConfig) (provider.Provider, error) {
		// Parse provider-specific configuration from the map
		providerCfg, err := LoadConfigFromMap(cfg.Name, cfg.ProviderConfig)
		if err != nil {
			return nil, err
		}

		// TLS settings (custom CA, mTLS, SNI, skip-verify) are framework-wide
		// via cfg.HTTP.TLS. Webhook keeps its own timeout knob in providerCfg;
		// the framework HTTP timeout is the fallback.
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
