// Package kubernetes implements Kubernetes resource watching for real-time DNS updates.
//
// The package uses client-go informers to watch Kubernetes resources (Ingress,
// IngressRoute, HTTPRoute, Service) and triggers reconciliation when workloads
// change. It supports both in-cluster and kubeconfig-based authentication.
//
// Key features:
//   - SharedInformerFactory for core resources (Ingress, Service)
//   - DynamicSharedInformerFactory for CRDs (IngressRoute, HTTPRoute)
//   - CRD auto-detection (checks API groups before starting informers)
//   - Event debouncing to prevent rapid-fire reconciliations
//   - Namespace filtering
//   - Label selector support
package kubernetes

import "time"

// Default configuration values.
const (
	// DefaultDebounceInterval is the time to wait for additional events
	// before triggering reconciliation.
	DefaultDebounceInterval = 2 * time.Second

	// DefaultResyncInterval is the interval for full informer re-list.
	// Zero means rely on watch events only.
	DefaultResyncInterval = 0
)

// Config holds Kubernetes watcher configuration.
type Config struct {
	// Kubeconfig is the path to the kubeconfig file.
	// Empty string means use in-cluster configuration (ServiceAccount).
	Kubeconfig string

	// Namespaces to watch. Empty means all namespaces.
	Namespaces []string

	// Resource types to watch.
	WatchIngress      bool // networking.k8s.io/v1 Ingress
	WatchIngressRoute bool // traefik.io/v1alpha1 IngressRoute
	WatchHTTPRoute    bool // gateway.networking.k8s.io/v1 HTTPRoute
	WatchServices     bool // v1 Service (opt-in, can be noisy)

	// LabelSelector filters resources by Kubernetes label selector.
	// Uses standard label selector syntax (e.g., "app=myapp,env=prod").
	// Empty string means no filtering.
	LabelSelector string

	// AnnotationFilter only processes resources with this annotation.
	// Format: "key=value" (e.g., "dnsweaver.dev/enabled=true").
	// Empty string means no filtering (all resources processed).
	AnnotationFilter string

	// DebounceInterval is the time to wait for additional events before
	// triggering reconciliation. This prevents rapid-fire reconciliations
	// during deployments or rollouts. Default: 2s.
	DebounceInterval time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
//
// By default, watches Ingress, IngressRoute, and HTTPRoute resources.
// Service watching is opt-in since it can generate significant event volume.
func DefaultConfig() Config {
	return Config{
		WatchIngress:      true,
		WatchIngressRoute: true,
		WatchHTTPRoute:    true,
		WatchServices:     false,
		DebounceInterval:  DefaultDebounceInterval,
	}
}

// HasNamespaceFilter returns true if specific namespaces are configured.
func (c Config) HasNamespaceFilter() bool {
	return len(c.Namespaces) > 0
}

// WatchesAnything returns true if at least one resource type is enabled.
func (c Config) WatchesAnything() bool {
	return c.WatchIngress || c.WatchIngressRoute || c.WatchHTTPRoute || c.WatchServices
}
