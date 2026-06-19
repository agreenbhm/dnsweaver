package powerdns

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// Provider implements provider.Provider for the PowerDNS Authoritative API.
type Provider struct {
	name       string
	zone       string
	serverID   string
	ttl        int
	client     *Client
	logger     *slog.Logger
	httpClient *http.Client
}

// ProviderOption is a functional option for configuring the Provider.
type ProviderOption func(*Provider)

// WithProviderLogger sets a custom logger for the provider.
func WithProviderLogger(logger *slog.Logger) ProviderOption {
	return func(p *Provider) {
		if logger != nil {
			p.logger = logger
		}
	}
}

// WithProviderHTTPClient sets a pre-configured HTTP client (shared TLS/timeout/UA).
func WithProviderHTTPClient(client *http.Client) ProviderOption {
	return func(p *Provider) {
		if client != nil {
			p.httpClient = client
		}
	}
}

// New creates a new PowerDNS provider instance.
func New(name string, config *Config, opts ...ProviderOption) (*Provider, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	p := &Provider{
		name:     name,
		zone:     config.Zone,
		serverID: config.ServerID,
		ttl:      config.TTL,
		logger:   slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}

	clientOpts := []ClientOption{WithLogger(p.logger)}
	if p.httpClient != nil {
		clientOpts = append(clientOpts, WithHTTPClient(p.httpClient))
	}
	p.client = NewClient(config.URL, config.APIKey, config.ServerID, clientOpts...)
	return p, nil
}

// NewFromEnv creates a PowerDNS provider from environment variables.
// Convenience constructor matching the provider-adding guide convention.
func NewFromEnv(instanceName string, opts ...ProviderOption) (*Provider, error) {
	config, err := LoadConfig(instanceName)
	if err != nil {
		return nil, err
	}
	return New(instanceName, config, opts...)
}

// NewFromMap creates a PowerDNS provider from a configuration map.
// Convenience constructor matching the provider-adding guide convention. The
// registry path uses Factory (which injects the shared HTTP client) instead.
func NewFromMap(name string, config map[string]string) (*Provider, error) {
	cfg, err := LoadConfigFromMap(name, config)
	if err != nil {
		return nil, err
	}
	return New(name, cfg)
}

// Name returns the provider instance name.
func (p *Provider) Name() string { return p.name }

// Type returns "powerdns".
func (p *Provider) Type() string { return "powerdns" }

// Capabilities returns the provider's feature support.
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsOwnershipTXT: true,
		SupportsNativeUpdate: true,
		SupportedRecordTypes: []provider.RecordType{
			provider.RecordTypeA,
			provider.RecordTypeAAAA,
			provider.RecordTypeCNAME,
			provider.RecordTypeSRV,
			provider.RecordTypeTXT,
		},
	}
}

// Identity returns the backend identity for multi-instance collision detection.
func (p *Provider) Identity() provider.ProviderIdentity {
	return provider.ProviderIdentity{
		Type:     "powerdns",
		Endpoint: p.client.baseURL,
		Zone:     p.zone,
	}
}

// Ping checks API connectivity/auth (via the client) and enforces that the
// configured zone pre-exists. dnsweaver never creates zones.
func (p *Provider) Ping(ctx context.Context) error {
	if err := p.client.Ping(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	if _, err := p.client.GetZone(ctx, p.zone); err != nil {
		if errors.Is(err, errZoneNotFound) {
			return fmt.Errorf("zone %q not found on PowerDNS server %q; create the zone before using it with dnsweaver", p.zone, p.serverID)
		}
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}
