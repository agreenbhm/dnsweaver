// Package dnsmasq implements the DNSWeaver provider interface for dnsmasq DNS server.
package dnsmasq

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/maxfield-allison/dnsweaver/pkg/provider"
)

// Provider implements provider.Provider for dnsmasq DNS server.
type Provider struct {
	name          string
	configPath    string // ConfigDir/ConfigFile, recorded for Identity reporting
	sshHost       string // SSH host for remote dnsmasq (empty = local)
	zone          string
	ttl           int
	reloadOnWrite bool
	client        *Client
	transport     *sshTransport // non-nil when SSH mode is active (owns the connection)
	logger        *slog.Logger
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

// WithReloadOnWrite enables automatic dnsmasq reload after writes.
// Default is true.
func WithReloadOnWrite(reload bool) ProviderOption {
	return func(p *Provider) {
		p.reloadOnWrite = reload
	}
}

// WithClient sets a custom client (for testing).
func WithClient(client *Client) ProviderOption {
	return func(p *Provider) {
		p.client = client
	}
}

// New creates a new dnsmasq provider instance.
func New(name string, config *Config, opts ...ProviderOption) (*Provider, error) {
	if config == nil {
		return nil, fmt.Errorf("config is required")
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	p := &Provider{
		name:          name,
		configPath:    config.ConfigDir + "/" + config.ConfigFile,
		sshHost:       config.SSHHost,
		zone:          config.Zone,
		ttl:           config.TTL,
		reloadOnWrite: true, // Default: reload after writes
		logger:        slog.Default(),
	}

	for _, opt := range opts {
		opt(p)
	}

	// Create client if not provided via options (testing)
	if p.client == nil {
		clientOpts := []ClientOption{WithLogger(p.logger)}

		// When SSH mode is enabled, establish the remote transport now and back
		// the client with it. Connecting here is intentional fail-fast behavior:
		// a configured-but-unreachable remote returns an error instead of
		// silently writing files and running the reload command locally.
		if config.IsSSHEnabled() {
			transport, err := newSSHTransport(context.Background(), config, p.logger)
			if err != nil {
				return nil, fmt.Errorf("dnsmasq provider %q: SSH mode is configured but the transport could not be established: %w", name, err)
			}
			p.transport = transport
			clientOpts = append(clientOpts, WithFileSystem(transport), WithCommandRunner(transport))
		}

		p.client = NewClient(
			config.ConfigDir,
			config.ConfigFile,
			config.ReloadCommand,
			config.Zone,
			clientOpts...,
		)
	}

	return p, nil
}

// NewFromEnv creates a new dnsmasq provider from environment variables.
// This is a convenience function for use with the provider registry.
func NewFromEnv(instanceName string, opts ...ProviderOption) (*Provider, error) {
	config, err := LoadConfig(instanceName)
	if err != nil {
		return nil, err
	}

	return New(instanceName, config, opts...)
}

// NewFromMap creates a new dnsmasq provider from a configuration map.
// This is used by the provider registry Factory pattern.
func NewFromMap(name string, config map[string]string) (*Provider, error) {
	cfg, err := LoadConfigFromMap(name, config)
	if err != nil {
		return nil, err
	}

	return New(name, cfg)
}

// Name returns the provider instance name.
func (p *Provider) Name() string {
	return p.name
}

// Type returns "dnsmasq".
func (p *Provider) Type() string {
	return "dnsmasq"
}

// Identity returns the backend identity for this provider instance.
// Two dnsmasq instances are considered the same backend when they write to
// the same config file on the same host. The endpoint encodes both the SSH
// host (or "local" for local writes) and the absolute config path.
// See provider.ProviderIdentity, issue #88.
func (p *Provider) Identity() provider.ProviderIdentity {
	host := p.sshHost
	if host == "" {
		host = "local"
	}
	return provider.ProviderIdentity{
		Type:     "dnsmasq",
		Endpoint: host + ":" + p.configPath,
		Zone:     p.zone,
	}
}

// Capabilities returns the provider's feature support.
// dnsmasq is file-based: no TXT ownership (files can't store arbitrary TXT records),
// no native update (file rewrite), only A and CNAME records.
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsOwnershipTXT: false, // File-based, can't store ownership TXT
		SupportsNativeUpdate: false, // Requires file rewrite (delete+create)
		SupportedRecordTypes: []provider.RecordType{
			provider.RecordTypeA,
			provider.RecordTypeCNAME,
		},
	}
}

// Zone returns the configured DNS zone.
func (p *Provider) Zone() string {
	return p.zone
}

// Ping checks connectivity to the dnsmasq configuration.
func (p *Provider) Ping(ctx context.Context) error {
	return p.client.Ping(ctx)
}

// List returns all managed records from the dnsmasq config file.
func (p *Provider) List(ctx context.Context) ([]provider.Record, error) {
	dnsmasqRecords, err := p.client.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing records: %w", err)
	}

	var records []provider.Record
	for _, r := range dnsmasqRecords {
		records = append(records, provider.Record{
			Hostname:   r.Hostname,
			Type:       r.Type,
			Target:     r.Target,
			TTL:        p.ttl, // dnsmasq doesn't use TTL, but we track it for consistency
			ProviderID: fmt.Sprintf("%s:%s:%s", r.Hostname, r.Type, r.Target),
		})
	}

	p.logger.Debug("listed records",
		slog.String("provider", p.name),
		slog.Int("count", len(records)),
	)

	return records, nil
}

// Create adds a new DNS record to the dnsmasq config.
func (p *Provider) Create(ctx context.Context, record provider.Record) error {
	// Validate record type
	switch record.Type {
	case provider.RecordTypeA, provider.RecordTypeAAAA, provider.RecordTypeCNAME:
		// Supported
	case provider.RecordTypeTXT:
		// dnsmasq supports txt-record= directive, but it's rarely needed
		// For now, skip TXT records (ownership tracking uses different mechanism)
		p.logger.Debug("skipping TXT record (not supported by dnsmasq provider)",
			slog.String("hostname", record.Hostname))
		return nil
	case provider.RecordTypeSRV:
		// dnsmasq supports srv-host= directive
		// SRV support deferred to post-v1.0 (#133)
		return fmt.Errorf("SRV records not yet supported by dnsmasq provider")
	default:
		return fmt.Errorf("unsupported record type: %s", record.Type)
	}

	rec := dnsmasqRecord{
		Hostname: record.Hostname,
		Type:     record.Type,
		Target:   record.Target,
	}

	if err := p.client.Create(ctx, rec); err != nil {
		return fmt.Errorf("creating %s record: %w", record.Type, err)
	}

	// Reload dnsmasq if configured
	if p.reloadOnWrite {
		if err := p.client.Reload(ctx); err != nil {
			p.logger.Warn("failed to reload dnsmasq",
				slog.String("error", err.Error()))
			// Don't fail the create, just warn
		}
	}

	p.logger.Info("created record",
		slog.String("provider", p.name),
		slog.String("hostname", record.Hostname),
		slog.String("type", string(record.Type)),
		slog.String("target", record.Target),
	)

	return nil
}

// Delete removes a DNS record from the dnsmasq config.
func (p *Provider) Delete(ctx context.Context, record provider.Record) error {
	// Skip TXT records (not supported)
	if record.Type == provider.RecordTypeTXT {
		p.logger.Debug("skipping TXT record deletion (not supported by dnsmasq provider)",
			slog.String("hostname", record.Hostname))
		return nil
	}

	rec := dnsmasqRecord{
		Hostname: record.Hostname,
		Type:     record.Type,
		Target:   record.Target,
	}

	if err := p.client.Delete(ctx, rec); err != nil {
		return fmt.Errorf("deleting %s record: %w", record.Type, err)
	}

	// Reload dnsmasq if configured
	if p.reloadOnWrite {
		if err := p.client.Reload(ctx); err != nil {
			p.logger.Warn("failed to reload dnsmasq",
				slog.String("error", err.Error()))
			// Don't fail the delete, just warn
		}
	}

	p.logger.Info("deleted record",
		slog.String("provider", p.name),
		slog.String("hostname", record.Hostname),
		slog.String("type", string(record.Type)),
	)

	return nil
}

// Close releases resources held by the provider. For SSH-backed instances this
// tears down the SFTP and SSH sessions. Local (file-based) instances hold no
// resources and Close is a no-op. Safe to call multiple times.
func (p *Provider) Close() error {
	if p.transport != nil {
		return p.transport.Close()
	}
	return nil
}

// Ensure Provider implements provider.Provider at compile time.
var _ provider.Provider = (*Provider)(nil)
