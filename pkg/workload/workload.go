// Package workload defines platform-agnostic types for representing resources
// that declare DNS hostnames. This abstraction enables dnsweaver to work with
// Docker containers, Kubernetes Ingress/IngressRoute/HTTPRoute/Services, and
// static configuration sources through a unified interface.
//
// The core type is [Workload], which represents any resource (container,
// K8s Ingress, etc.) that may declare hostnames for DNS record management.
// Sources extract hostnames from Workloads, and the reconciler operates
// on Workloads regardless of their origin platform.
//
// Example usage:
//
//	// Docker adapter converts docker.Workload → workload.Workload
//	w := workload.Workload{
//	    ID:       container.ID,
//	    Name:     container.Name,
//	    Labels:   container.Labels,
//	    Platform: workload.PlatformDocker,
//	    Kind:     workload.KindContainer,
//	}
//
//	// K8s adapter converts Ingress → workload.Workload
//	w := workload.Workload{
//	    ID:          string(ingress.UID),
//	    Name:        ingress.Namespace + "/" + ingress.Name,
//	    Namespace:   ingress.Namespace,
//	    Labels:      ingress.Labels,
//	    Annotations: ingress.Annotations,
//	    Platform:    workload.PlatformKubernetes,
//	    Kind:        workload.KindIngress,
//	    Hostnames:   extractedHosts,
//	}
package workload

import "context"

// Platform identifies the workload's origin platform.
type Platform string

const (
	// PlatformDocker represents workloads from Docker (Swarm or standalone).
	PlatformDocker Platform = "docker"
	// PlatformKubernetes represents workloads from Kubernetes.
	PlatformKubernetes Platform = "kubernetes"
	// PlatformStatic represents workloads from static file configuration.
	PlatformStatic Platform = "static"
)

// String returns the string representation of the platform.
func (p Platform) String() string {
	return string(p)
}

// Kind identifies the specific resource type within a platform.
type Kind string

const (
	// Docker kinds.

	// KindContainer represents a standalone Docker container.
	KindContainer Kind = "container"
	// KindService represents a Docker Swarm service.
	KindService Kind = "service"

	// Kubernetes kinds.

	// KindIngress represents a networking.k8s.io/v1 Ingress.
	KindIngress Kind = "ingress"
	// KindIngressRoute represents a traefik.io/v1alpha1 IngressRoute.
	KindIngressRoute Kind = "ingressroute"
	// KindHTTPRoute represents a gateway.networking.k8s.io/v1 HTTPRoute.
	KindHTTPRoute Kind = "httproute"
	// KindK8sService represents a v1 Service with hostname annotations.
	KindK8sService Kind = "k8s-service"
	// KindPod represents a v1 Pod with hostname annotations.
	KindPod Kind = "pod"
)

// String returns the string representation of the kind.
func (k Kind) String() string {
	return string(k)
}

// Workload is a platform-agnostic representation of something that declares
// DNS hostnames. It could be a Docker container, a Kubernetes Ingress, a
// Traefik IngressRoute, or any other resource that expresses intent for
// DNS record management.
//
// Sources read Labels, Annotations, and Hostnames from the Workload to extract
// hostname information. The reconciler operates on Workloads without knowing
// which platform they originated from.
type Workload struct {
	// ID is a unique identifier (container ID, K8s UID, etc.).
	ID string

	// Name is human-readable (container name, "namespace/name", etc.).
	Name string

	// Namespace is the Kubernetes namespace. Empty for Docker workloads.
	Namespace string

	// Labels from the resource (Docker labels, K8s labels).
	Labels map[string]string

	// Annotations from the resource (K8s annotations; Docker does not use these).
	// Sources check both Labels and Annotations for hostname extraction.
	Annotations map[string]string

	// Platform identifies the origin (docker, kubernetes, static).
	Platform Platform

	// Kind identifies the specific resource type.
	Kind Kind

	// Hostnames are pre-extracted hostnames for resources that declare them
	// structurally (Ingress rules, IngressRoute Host() matchers, HTTPRoute
	// hostnames). When non-empty, sources can use these directly instead of
	// parsing labels. This handles resources where "presence = intent" is
	// expressed in the resource spec, not in labels.
	Hostnames []string

	// Metadata holds platform-specific data that sources might need.
	// For K8s: resource version, generation, etc.
	// For Docker: container state, image name, etc.
	Metadata map[string]string
}

// String returns a human-readable representation of the workload.
func (w Workload) String() string {
	return string(w.Platform) + "/" + string(w.Kind) + ":" + w.Name
}

// HasLabel returns true if the workload has the specified label.
func (w Workload) HasLabel(key string) bool {
	if w.Labels == nil {
		return false
	}
	_, ok := w.Labels[key]
	return ok
}

// GetLabel returns the value of the specified label, or empty string if not found.
func (w Workload) GetLabel(key string) string {
	if w.Labels == nil {
		return ""
	}
	return w.Labels[key]
}

// GetLabelOr returns the value of the specified label, or the default if not found.
func (w Workload) GetLabelOr(key, defaultValue string) string {
	if w.Labels != nil {
		if v, ok := w.Labels[key]; ok {
			return v
		}
	}
	return defaultValue
}

// HasAnnotation returns true if the workload has the specified annotation.
func (w Workload) HasAnnotation(key string) bool {
	if w.Annotations == nil {
		return false
	}
	_, ok := w.Annotations[key]
	return ok
}

// GetAnnotation returns the value of the specified annotation, or empty string if not found.
func (w Workload) GetAnnotation(key string) string {
	if w.Annotations == nil {
		return ""
	}
	return w.Annotations[key]
}

// GetAnnotationOr returns the annotation value, or the default if not found.
func (w Workload) GetAnnotationOr(key, defaultValue string) string {
	if w.Annotations != nil {
		if v, ok := w.Annotations[key]; ok {
			return v
		}
	}
	return defaultValue
}

// GetLabelOrAnnotation checks labels first, then annotations.
// This is useful for sources that accept configuration from either location,
// enabling compatibility with both Docker labels and Kubernetes annotations.
func (w Workload) GetLabelOrAnnotation(key string) string {
	if w.Labels != nil {
		if v, ok := w.Labels[key]; ok {
			return v
		}
	}
	if w.Annotations != nil {
		if v, ok := w.Annotations[key]; ok {
			return v
		}
	}
	return ""
}

// IsDocker returns true if this workload originated from Docker.
func (w Workload) IsDocker() bool {
	return w.Platform == PlatformDocker
}

// IsKubernetes returns true if this workload originated from Kubernetes.
func (w Workload) IsKubernetes() bool {
	return w.Platform == PlatformKubernetes
}

// Lister is the interface for listing workloads from a platform.
// Each platform (Docker, Kubernetes, static) implements this interface.
// The reconciler aggregates workloads from all registered listers.
type Lister interface {
	// ListWorkloads returns all workloads from this platform.
	ListWorkloads(ctx context.Context) ([]Workload, error)

	// Platform returns the platform this lister covers.
	Platform() Platform
}
