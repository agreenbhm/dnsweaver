// Package proxmox provides a Source implementation for extracting hostnames
// from Proxmox VE workloads (VMs and LXC containers).
//
// This source constructs DNS A records mapping each running Proxmox resource
// to its resolved IP address. Two hostname strategies are supported:
//
//  1. Domain suffix (recommended): Set a domain via WithDomain("home.example.com").
//     Each resource is registered as "<vm-name>.<domain>" (e.g., webserver.home.example.com).
//
//  2. Bare FQDN: If the VM name already contains a dot, it is used directly
//     as the hostname without appending a domain suffix.
//
// The resolved IP (populated in w.Metadata["ip"] by the adapter) is used as
// the A record target. Resources with no resolved IP are skipped.
//
// Supported workload platforms: [PlatformProxmox]
package proxmox

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/source"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/workload"
)

const sourceName = "proxmox"

// Proxmox implements the source.Source interface for Proxmox VE workloads.
type Proxmox struct {
	domain string
	logger *slog.Logger
}

// Option is a functional option for configuring Proxmox.
type Option func(*Proxmox)

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) Option {
	return func(p *Proxmox) {
		p.logger = logger
	}
}

// WithDomain sets the DNS domain suffix used when constructing hostnames from
// VM names. For example, WithDomain("home.example.com") causes a VM named
// "webserver" to be registered as "webserver.home.example.com".
//
// If not set (or set to empty string), only VM names that already contain a dot
// are used as hostnames. Plain VM names without a domain suffix are skipped.
func WithDomain(domain string) Option {
	return func(p *Proxmox) {
		p.domain = strings.TrimPrefix(strings.TrimSpace(domain), ".")
	}
}

// New creates a new Proxmox source.
func New(opts ...Option) *Proxmox {
	p := &Proxmox{
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name returns the source identifier.
func (p *Proxmox) Name() string {
	return sourceName
}

// SupportedPlatforms returns the platforms this source handles.
// The Proxmox source only processes workloads with PlatformProxmox.
func (p *Proxmox) SupportedPlatforms() []workload.Platform {
	return []workload.Platform{workload.PlatformProxmox}
}

// SupportsDiscovery returns false — the Proxmox source does not support
// static file discovery. All hostnames come from live workload extraction.
func (p *Proxmox) SupportsDiscovery() bool {
	return false
}

// Discover is a no-op for the Proxmox source.
func (p *Proxmox) Discover(_ context.Context) ([]source.Hostname, error) {
	return nil, nil
}

// Extract builds DNS A records for a Proxmox workload.
//
// Extraction logic:
//  1. Skip non-Proxmox workloads (wrong platform).
//  2. Look up the resolved IP in w.Metadata["ip"]. Skip if empty.
//  3. Determine the hostname:
//     a. If w.Name contains a dot, use it directly (already FQDN).
//     b. If a domain suffix is configured, append: "<name>.<domain>".
//     c. Otherwise, skip (no valid hostname can be determined).
//  4. Return a single source.Hostname with an A record hint.
//
// Each returned Hostname has RecordHints.Type="A" and RecordHints.Target set to
// the resolved IP, allowing providers to create or update A records.
func (p *Proxmox) Extract(_ context.Context, w workload.Workload) ([]source.Hostname, error) {
	if !w.Platform.IsProxmox() {
		return nil, nil
	}

	ip := w.Metadata["ip"]
	if ip == "" {
		p.logger.Debug("proxmox workload has no resolved IP; skipping",
			slog.String("workload", w.Name),
			slog.String("node", w.Metadata["node"]),
		)
		return nil, nil
	}

	hostname, err := p.resolveHostname(w.Name)
	if err != nil {
		p.logger.Debug("could not determine hostname for proxmox workload; skipping",
			slog.String("workload", w.Name),
			slog.String("reason", err.Error()),
		)
		return nil, nil //nolint:nilerr // not a configuration error; silently skip
	}

	h := source.Hostname{
		Name:   hostname,
		Source: sourceName,
		Router: fmt.Sprintf("%s/%s", w.Metadata["node"], w.Metadata["vmid"]),
		RecordHints: &source.RecordHints{
			Type:   "A",
			Target: ip,
		},
	}

	return []source.Hostname{h}, nil
}

// resolveHostname determines the FQDN for a given VM name.
// Returns an error (logged as debug, not returned to caller) if no hostname can be determined.
func (p *Proxmox) resolveHostname(name string) (string, error) {
	if strings.Contains(name, ".") {
		// VM name already looks like an FQDN — use it directly.
		return name, nil
	}
	if p.domain != "" {
		return name + "." + p.domain, nil
	}
	return "", fmt.Errorf("VM name %q is not an FQDN and no domain suffix is configured", name)
}
