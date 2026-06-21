// Package kubernetes provides a Source implementation for extracting hostnames
// from Kubernetes resources. It reads pre-extracted hostnames from the workload
// (populated by resource converters in internal/kubernetes/resources/) and applies
// optional annotation-based record hints.
//
// Annotation-based configuration allows K8s users to control DNS record creation
// without custom labels. All annotations use the "dnsweaver.dev/" prefix.
//
// Resource-level annotations (applied to ALL hostnames on the resource):
//
//	dnsweaver.dev/enabled: "true"          # Opt-in/out of DNS management
//	dnsweaver.dev/record-type: A           # Override record type (A, AAAA, CNAME, SRV, TXT)
//	dnsweaver.dev/target: 10.0.0.100      # Override record target
//	dnsweaver.dev/ttl: "300"               # Override TTL in seconds
//	dnsweaver.dev/provider: internal-dns   # Route to specific provider instance
//	dnsweaver.dev/proxied: "true"          # Provider-specific (e.g., Cloudflare proxy)
package kubernetes

import (
	"strconv"
	"strings"

	"github.com/maxfield-allison/dnsweaver/pkg/source"
)

// Annotation key constants for the dnsweaver.dev/ prefix.
const (
	// AnnotationPrefix is the base prefix for all dnsweaver K8s annotations.
	AnnotationPrefix = "dnsweaver.dev/"

	// AnnotationEnabled controls whether dnsweaver manages DNS for this resource.
	// Set to "false" to skip. Default is "true" (managed).
	AnnotationEnabled = AnnotationPrefix + "enabled"

	// AnnotationRecordType overrides the DNS record type (A, AAAA, CNAME, SRV, TXT).
	AnnotationRecordType = AnnotationPrefix + "record-type"

	// AnnotationTarget overrides the DNS record target value.
	AnnotationTarget = AnnotationPrefix + "target"

	// AnnotationTTL overrides the DNS record TTL in seconds.
	AnnotationTTL = AnnotationPrefix + "ttl"

	// AnnotationProvider routes the record to a specific provider instance by name.
	AnnotationProvider = AnnotationPrefix + "provider"

	// AnnotationProxied is a provider-specific hint (e.g., Cloudflare proxy mode).
	AnnotationProxied = AnnotationPrefix + "proxied"
)

// isEnabled checks whether dnsweaver management is enabled for this resource.
// Returns true if the annotation is absent or set to anything other than "false".
func isEnabled(annotations map[string]string) bool {
	if annotations == nil {
		return true
	}
	v, ok := annotations[AnnotationEnabled]
	if !ok {
		return true
	}
	return !strings.EqualFold(v, "false")
}

// parseRecordHints extracts RecordHints from dnsweaver.dev/* annotations.
// Returns nil if no hint annotations are present.
func parseRecordHints(annotations map[string]string) *source.RecordHints {
	if annotations == nil {
		return nil
	}

	var hints source.RecordHints
	var hasAny bool

	if v, ok := annotations[AnnotationRecordType]; ok && v != "" {
		hints.Type = strings.ToUpper(v)
		hasAny = true
	}

	if v, ok := annotations[AnnotationTarget]; ok && v != "" {
		hints.Target = v
		hasAny = true
	}

	if v, ok := annotations[AnnotationTTL]; ok && v != "" {
		if ttl, err := strconv.Atoi(v); err == nil && ttl > 0 {
			hints.TTL = ttl
			hasAny = true
		}
	}

	if v, ok := annotations[AnnotationProvider]; ok && v != "" {
		hints.Provider = v
		hasAny = true
	}

	// Provider-specific metadata (proxied, etc.)
	if v, ok := annotations[AnnotationProxied]; ok && v != "" {
		if hints.Metadata == nil {
			hints.Metadata = make(map[string]string)
		}
		hints.Metadata["proxied"] = v
		hasAny = true
	}

	if !hasAny {
		return nil
	}
	return &hints
}
