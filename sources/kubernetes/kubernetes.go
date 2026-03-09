// Package kubernetes provides a Source implementation for extracting hostnames
// from Kubernetes workloads.
//
// This source reads pre-extracted hostnames from the workload (populated by
// resource converters in internal/kubernetes/resources/) and converts them to
// source.Hostname values with optional RecordHints from annotations.
//
// Architecture note: Hostname extraction from K8s specs is performed by the
// resource converters (ConvertIngress, ConvertIngressRoute, ConvertHTTPRoute,
// ConvertService). This source acts as the bridge between the converter output
// (workload.Workload.Hostnames) and the source registry's extraction pipeline.
//
// Supported resources:
//   - networking.k8s.io/v1 Ingress — .spec.rules[].host, .spec.tls[].hosts[]
//   - traefik.io/v1alpha1 IngressRoute — Host() matchers in .spec.routes[].match
//   - gateway.networking.k8s.io/v1 HTTPRoute — .spec.hostnames[]
//   - v1 Service — external-dns and dnsweaver.dev/* hostname annotations
//
// All resources support optional dnsweaver.dev/* annotations for RecordHints.
package kubernetes

import (
	"context"
	"log/slog"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/source"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/workload"
)

const sourceName = "kubernetes"

// Kubernetes implements the source.Source interface for extracting hostnames
// from Kubernetes workloads. It reads pre-extracted hostnames and applies
// annotation-based record hints.
type Kubernetes struct {
	logger *slog.Logger
}

// Option is a functional option for configuring Kubernetes.
type Option func(*Kubernetes)

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) Option {
	return func(k *Kubernetes) {
		k.logger = logger
	}
}

// New creates a new Kubernetes source.
func New(opts ...Option) *Kubernetes {
	k := &Kubernetes{
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

// Name returns the source identifier.
func (k *Kubernetes) Name() string {
	return sourceName
}

// Extract reads pre-extracted hostnames from the workload and converts them
// to source.Hostname values with optional RecordHints from annotations.
//
// The extraction pipeline:
//  1. Check dnsweaver.dev/enabled annotation — skip if "false"
//  2. Read w.Hostnames (pre-populated by resource converters)
//  3. Parse dnsweaver.dev/* annotations for resource-level RecordHints
//  4. Return source.Hostname for each hostname with hints attached
//
// Returns an empty slice if no hostnames are found or the resource is disabled.
func (k *Kubernetes) Extract(_ context.Context, w workload.Workload) ([]source.Hostname, error) {
	// Check if this resource has opted out of DNS management.
	if !isEnabled(w.Annotations) {
		k.logger.Debug("resource disabled via annotation",
			slog.String("workload", w.Name),
		)
		return nil, nil
	}

	// If no pre-extracted hostnames, nothing to do.
	if len(w.Hostnames) == 0 {
		return nil, nil
	}

	// Parse resource-level record hints from annotations.
	hints := parseRecordHints(w.Annotations)

	// Build the router identifier from the workload kind and name for attribution.
	router := string(w.Kind)
	if w.Name != "" {
		router = string(w.Kind) + ":" + w.Name
	}

	hostnames := make([]source.Hostname, 0, len(w.Hostnames))
	for _, name := range w.Hostnames {
		if name == "" {
			continue
		}

		h := source.Hostname{
			Name:   name,
			Source: sourceName,
			Router: router,
		}

		// Apply resource-level hints to each hostname.
		// Each hostname gets its own copy to avoid shared mutation.
		if hints != nil {
			hintsCopy := *hints
			if hints.Metadata != nil {
				hintsCopy.Metadata = make(map[string]string, len(hints.Metadata))
				for mk, mv := range hints.Metadata {
					hintsCopy.Metadata[mk] = mv
				}
			}
			h.RecordHints = &hintsCopy
		}

		hostnames = append(hostnames, h)
	}

	if len(hostnames) > 0 {
		k.logger.Debug("extracted hostnames from kubernetes workload",
			slog.String("workload", w.Name),
			slog.String("kind", string(w.Kind)),
			slog.Int("count", len(hostnames)),
		)
	}

	return hostnames, nil
}

// Discover is not supported for Kubernetes resources.
// K8s hostnames come from the informer cache via Extract, not static files.
func (k *Kubernetes) Discover(_ context.Context) ([]source.Hostname, error) {
	return nil, nil
}

// SupportsDiscovery returns false — K8s resources don't use file discovery.
func (k *Kubernetes) SupportsDiscovery() bool {
	return false
}

// SupportedPlatforms returns [PlatformKubernetes] — this source only handles
// Kubernetes workloads. Docker workloads are handled by other sources.
func (k *Kubernetes) SupportedPlatforms() []workload.Platform {
	return []workload.Platform{workload.PlatformKubernetes}
}
