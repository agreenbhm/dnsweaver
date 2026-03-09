// Package resources provides functions for converting Kubernetes objects
// into platform-agnostic workload.Workload values.
//
// Each converter extracts hostnames and metadata from a specific Kubernetes
// resource type. Typed converters are used for core resources (Ingress, Service),
// while unstructured converters handle CRDs (IngressRoute, HTTPRoute).
package resources

import "strings"

// splitCSV splits a comma-separated string into trimmed, non-empty values.
func splitCSV(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
