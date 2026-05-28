package adguard

import (
	"gitlab.bluewillows.net/root/dnsweaver/pkg/httputil"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// Factory returns a provider.Factory for creating AdGuard Home provider instances.
func Factory() provider.Factory {
	return func(cfg provider.FactoryConfig) (provider.Provider, error) {
		providerCfg, err := LoadConfigFromMap(cfg.Name, cfg.ProviderConfig)
		if err != nil {
			return nil, err
		}

		// TLS settings (custom CA, mTLS, SNI, skip-verify) are configured
		// framework-wide via cfg.HTTP.TLS. httputil.NewClient itself logs
		// a WARN when verification is skipped, so providers no longer need
		// to duplicate that message.
		httpClient := httputil.NewClient(&httputil.ClientConfig{
			Timeout:   cfg.HTTP.Timeout,
			TLS:       cfg.HTTP.TLS,
			UserAgent: cfg.HTTP.UserAgent,
			Logger:    cfg.HTTP.Logger,
		})

		_ = providerCfg.URL // referenced indirectly via WithProviderHTTPClient

		return New(cfg.Name, providerCfg,
			WithProviderHTTPClient(httpClient),
			WithProviderLogger(cfg.HTTP.Logger),
		)
	}
}
