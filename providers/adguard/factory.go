package adguard

import (
	"log/slog"

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

		httpClient := httputil.NewClient(&httputil.ClientConfig{
			Timeout:       cfg.HTTP.Timeout,
			TLSSkipVerify: cfg.HTTP.TLSSkipVerify,
			UserAgent:     cfg.HTTP.UserAgent,
			Logger:        cfg.HTTP.Logger,
		})

		if cfg.HTTP.TLSSkipVerify && cfg.HTTP.Logger != nil {
			cfg.HTTP.Logger.Warn("TLS certificate verification disabled for AdGuard Home provider",
				slog.String("provider", cfg.Name),
				slog.String("url", providerCfg.URL),
			)
		}

		return New(cfg.Name, providerCfg,
			WithProviderHTTPClient(httpClient),
			WithProviderLogger(cfg.HTTP.Logger),
		)
	}
}
