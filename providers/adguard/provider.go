package adguard

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// Provider implements provider.Provider and provider.Updater for AdGuard Home DNS.
// It manages DNS records via AdGuard Home's DNS Rewrite API.
type Provider struct {
	name   string
	url    string // AdGuard Home URL (recorded for Identity reporting)
	zone   string
	ttl    int
	client *Client
	logger *slog.Logger
}

// Compile-time check that Provider implements the required interfaces.
var (
	_ provider.Provider = (*Provider)(nil)
	_ provider.Updater  = (*Provider)(nil)
)

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

// WithProviderHTTPClient sets a custom HTTP client for the provider.
func WithProviderHTTPClient(client *http.Client) ProviderOption {
	return func(p *Provider) {
		if client != nil && p.client != nil {
			p.client.httpClient = client
		}
	}
}

// WithClient sets a custom API client (for testing).
func WithClient(client *Client) ProviderOption {
	return func(p *Provider) {
		p.client = client
	}
}

// New creates a new AdGuard Home provider instance.
func New(name string, config *Config, opts ...ProviderOption) (*Provider, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	p := &Provider{
		name:   name,
		url:    config.URL,
		zone:   config.Zone,
		ttl:    config.TTL,
		logger: slog.Default(),
		client: NewClient(config.URL, config.Username, config.Password),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

// Name returns the provider instance name.
func (p *Provider) Name() string {
	return p.name
}

// Type returns "adguard".
func (p *Provider) Type() string {
	return "adguard"
}

// Identity returns the backend identity for this provider instance.
// Two adguard instances are considered the same backend when they target
// the same AdGuard Home URL and zone (see provider.ProviderIdentity, issue #88).
func (p *Provider) Identity() provider.ProviderIdentity {
	return provider.ProviderIdentity{
		Type:     "adguard",
		Endpoint: p.url,
		Zone:     p.zone,
	}
}

// Capabilities returns the provider's feature support.
// AdGuard Home rewrites support A, AAAA, and CNAME records.
// TXT records are not supported by the rewrite API, so ownership tracking is unavailable.
// Native update is supported via PUT /control/rewrite/update.
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsOwnershipTXT: false,
		SupportsNativeUpdate: true,
		SupportedRecordTypes: []provider.RecordType{
			provider.RecordTypeA,
			provider.RecordTypeAAAA,
			provider.RecordTypeCNAME,
		},
	}
}

// Ping checks connectivity to AdGuard Home.
func (p *Provider) Ping(ctx context.Context) error {
	return p.client.Ping(ctx)
}

// List returns all managed records from AdGuard Home.
func (p *Provider) List(ctx context.Context) ([]provider.Record, error) {
	entries, err := p.client.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing records: %w", err)
	}

	var records []provider.Record
	for _, e := range entries {
		// Skip disabled rewrites
		if e.Enabled != nil && !*e.Enabled {
			continue
		}

		// Determine record type from the answer value
		recordType := classifyAnswer(e.Answer)
		if recordType == "" {
			// Skip unsupported rewrite types (e.g., wildcard blocks like "A")
			continue
		}

		// Zone filtering: if zone is configured, only include matching records
		if p.zone != "" && !strings.HasSuffix(e.Domain, "."+p.zone) && e.Domain != p.zone {
			continue
		}

		records = append(records, provider.Record{
			Hostname:   e.Domain,
			Type:       recordType,
			Target:     e.Answer,
			TTL:        p.ttl,
			ProviderID: fmt.Sprintf("%s:%s", e.Domain, e.Answer),
		})
	}

	p.logger.Debug("listed records",
		slog.String("provider", p.name),
		slog.Int("total_rewrites", len(entries)),
		slog.Int("matched", len(records)),
	)

	return records, nil
}

// Create adds a new DNS record via AdGuard Home rewrite.
func (p *Provider) Create(ctx context.Context, record provider.Record) error {
	switch record.Type {
	case provider.RecordTypeA, provider.RecordTypeAAAA, provider.RecordTypeCNAME:
		// Supported
	case provider.RecordTypeTXT:
		// AdGuard Home doesn't support TXT rewrites; skip silently
		p.logger.Debug("skipping TXT record (not supported by AdGuard Home provider)",
			slog.String("hostname", record.Hostname))
		return nil
	default:
		return fmt.Errorf("%s records not supported by AdGuard Home provider", record.Type)
	}

	entry := rewriteEntry{
		Domain: record.Hostname,
		Answer: record.Target,
	}

	if err := p.client.Create(ctx, entry); err != nil {
		return fmt.Errorf("creating record %s -> %s: %w", record.Hostname, record.Target, err)
	}

	p.logger.Info("created DNS record",
		slog.String("provider", p.name),
		slog.String("hostname", record.Hostname),
		slog.String("type", string(record.Type)),
		slog.String("target", record.Target),
	)

	return nil
}

// Delete removes a DNS record via AdGuard Home rewrite.
func (p *Provider) Delete(ctx context.Context, record provider.Record) error {
	switch record.Type {
	case provider.RecordTypeA, provider.RecordTypeAAAA, provider.RecordTypeCNAME:
		// Supported
	case provider.RecordTypeTXT:
		// AdGuard Home doesn't support TXT rewrites; skip silently
		return nil
	default:
		return fmt.Errorf("%s records not supported by AdGuard Home provider", record.Type)
	}

	entry := rewriteEntry{
		Domain: record.Hostname,
		Answer: record.Target,
	}

	if err := p.client.Delete(ctx, entry); err != nil {
		return fmt.Errorf("deleting record %s -> %s: %w", record.Hostname, record.Target, err)
	}

	p.logger.Info("deleted DNS record",
		slog.String("provider", p.name),
		slog.String("hostname", record.Hostname),
		slog.String("type", string(record.Type)),
		slog.String("target", record.Target),
	)

	return nil
}

// Update modifies an existing DNS record in place.
func (p *Provider) Update(ctx context.Context, existing, desired provider.Record) error {
	target := rewriteEntry{
		Domain: existing.Hostname,
		Answer: existing.Target,
	}
	update := rewriteEntry{
		Domain: desired.Hostname,
		Answer: desired.Target,
	}

	if err := p.client.Update(ctx, target, update); err != nil {
		return fmt.Errorf("updating record %s -> %s: %w", desired.Hostname, desired.Target, err)
	}

	p.logger.Info("updated DNS record",
		slog.String("provider", p.name),
		slog.String("hostname", desired.Hostname),
		slog.String("type", string(desired.Type)),
		slog.String("old_target", existing.Target),
		slog.String("new_target", desired.Target),
	)

	return nil
}

// classifyAnswer determines the DNS record type from an AdGuard Home rewrite answer value.
// Returns empty string for unsupported answer types.
func classifyAnswer(answer string) provider.RecordType {
	// Check for IPv4 address → A record
	if ip := net.ParseIP(answer); ip != nil {
		if ip.To4() != nil {
			return provider.RecordTypeA
		}
		return provider.RecordTypeAAAA
	}

	// Non-IP answers are CNAME records (hostname targets)
	// AdGuard also uses special values like "" (empty) for blocking, skip those
	if answer == "" {
		return ""
	}

	return provider.RecordTypeCNAME
}
