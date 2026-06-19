package powerdns

import (
	"gitlab.bluewillows.net/root/dnsweaver/pkg/httputil"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// Factory returns a provider.Factory for creating PowerDNS provider instances.
func Factory() provider.Factory {
	return func(cfg provider.FactoryConfig) (provider.Provider, error) {
		providerCfg, err := LoadConfigFromMap(cfg.Name, cfg.ProviderConfig)
		if err != nil {
			return nil, err
		}

		// TLS settings are framework-wide via cfg.HTTP.TLS; httputil emits its
		// own WARN when verification is skipped.
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
