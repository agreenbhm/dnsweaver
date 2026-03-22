package testutil

import (
	"log/slog"

	"gitlab.bluewillows.net/root/dnsweaver/internal/matcher"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/source"
)

// MockProviderRegistry creates a provider registry pre-loaded with mock providers.
func MockProviderRegistry(logger *slog.Logger, mocks ...*MockProvider) *provider.Registry {
	if logger == nil {
		logger = QuietLogger()
	}
	reg := provider.NewRegistry(logger)

	reg.RegisterFactory("mock", func(cfg provider.FactoryConfig) (provider.Provider, error) {
		for _, m := range mocks {
			if m.Name() == cfg.Name {
				return m, nil
			}
		}
		return NewMockProvider(cfg.Name), nil
	})

	return reg
}

// MockProviderInstance creates a ProviderInstance wrapping a mock provider
// with the given domain includes and record type/target.
func MockProviderInstance(mock *MockProvider, domains []string, recordType provider.RecordType, target string) *provider.ProviderInstance {
	matcherCfg := matcher.DomainMatcherConfig{
		Includes: domains,
		Excludes: nil,
		UseRegex: false,
	}
	domainMatcher, _ := matcher.NewDomainMatcher(matcherCfg)

	return &provider.ProviderInstance{
		Provider:   mock,
		Matcher:    domainMatcher,
		RecordType: recordType,
		Target:     target,
		TTL:        DefaultTTL,
	}
}

// MockSourceRegistry creates a source registry pre-loaded with mock sources.
func MockSourceRegistry(logger *slog.Logger, sources ...*MockSource) *source.Registry {
	if logger == nil {
		logger = QuietLogger()
	}
	reg := source.NewRegistry(logger)
	for _, s := range sources {
		_ = reg.Register(s)
	}
	return reg
}
