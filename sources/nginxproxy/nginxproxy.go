// Package nginxproxy provides a Source implementation for extracting hostnames
// from nginx-proxy style Docker container labels.
//
// nginx-proxy (jwilder/nginx-proxy and compatible reverse proxies) declares
// hostnames via the VIRTUAL_HOST variable. Upstream nginx-proxy reads this
// from container environment variables; dnsweaver currently consumes Docker
// labels only, so this source recognizes the label-based forms:
//
//	VIRTUAL_HOST=app.example.com
//	VIRTUAL_HOST=app.example.com,www.example.com
//	com.nginx-proxy.virtual_host=app.example.com
//
// Env-var extraction is tracked separately — when Workload gains env-var
// support, this source can be extended without a breaking change.
package nginxproxy

import (
	"context"
	"log/slog"

	"github.com/maxfield-allison/dnsweaver/pkg/source"
	"github.com/maxfield-allison/dnsweaver/pkg/workload"
)

const sourceName = "nginx-proxy"

// NginxProxy implements source.Source for nginx-proxy style labels.
type NginxProxy struct {
	parser *Parser
	logger *slog.Logger
}

// Option configures a NginxProxy source.
type Option func(*NginxProxy)

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) Option {
	return func(n *NginxProxy) {
		n.logger = logger
	}
}

// New creates a new NginxProxy source.
func New(opts ...Option) *NginxProxy {
	n := &NginxProxy{logger: slog.Default()}
	for _, opt := range opts {
		opt(n)
	}
	n.parser = NewParser(WithParserLogger(n.logger))
	return n
}

// Name returns the source identifier.
func (n *NginxProxy) Name() string {
	return sourceName
}

// Extract reads recognized nginx-proxy labels and returns the declared
// hostnames. Comma-separated values are supported. Duplicates across
// labels are collapsed.
//
// Never returns an error — malformed values are silently skipped.
func (n *NginxProxy) Extract(_ context.Context, w workload.Workload) ([]source.Hostname, error) {
	if len(w.Labels) == 0 {
		return nil, nil
	}

	extractions := n.parser.ExtractHostnames(w.Labels)
	if len(extractions) == 0 {
		return nil, nil
	}

	hostnames := make([]source.Hostname, 0, len(extractions))
	for _, e := range extractions {
		hostnames = append(hostnames, source.Hostname{
			Name:   e.Hostname,
			Source: sourceName,
			Router: e.Router,
		})
	}

	n.logger.Debug("extracted hostnames from nginx-proxy labels",
		slog.Int("count", len(hostnames)),
	)

	return hostnames, nil
}

// Discover is a no-op — nginx static config discovery is not implemented.
func (n *NginxProxy) Discover(_ context.Context) ([]source.Hostname, error) {
	return nil, nil
}

// SupportsDiscovery always returns false.
func (n *NginxProxy) SupportsDiscovery() bool {
	return false
}

// SupportedPlatforms returns an empty slice — any label-exposing platform works.
func (n *NginxProxy) SupportedPlatforms() []workload.Platform {
	return nil
}
