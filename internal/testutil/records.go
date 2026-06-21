package testutil

import (
	"github.com/maxfield-allison/dnsweaver/pkg/provider"
	"github.com/maxfield-allison/dnsweaver/pkg/source"
)

// DefaultTTL is the default TTL for test records.
const DefaultTTL = 300

// --- DNS Record builders ---

// ARecord creates an A record for testing.
func ARecord(hostname, ip string) provider.Record {
	return provider.Record{
		Hostname: hostname,
		Type:     provider.RecordTypeA,
		Target:   ip,
		TTL:      DefaultTTL,
	}
}

// ARecordWithTTL creates an A record with a specific TTL.
func ARecordWithTTL(hostname, ip string, ttl int) provider.Record {
	return provider.Record{
		Hostname: hostname,
		Type:     provider.RecordTypeA,
		Target:   ip,
		TTL:      ttl,
	}
}

// AAAARecord creates an AAAA record for testing.
func AAAARecord(hostname, ip string) provider.Record {
	return provider.Record{
		Hostname: hostname,
		Type:     provider.RecordTypeAAAA,
		Target:   ip,
		TTL:      DefaultTTL,
	}
}

// CNAMERecord creates a CNAME record for testing.
func CNAMERecord(hostname, target string) provider.Record {
	return provider.Record{
		Hostname: hostname,
		Type:     provider.RecordTypeCNAME,
		Target:   target,
		TTL:      DefaultTTL,
	}
}

// TXTRecord creates a TXT record for testing.
func TXTRecord(hostname, value string) provider.Record {
	return provider.Record{
		Hostname: hostname,
		Type:     provider.RecordTypeTXT,
		Target:   value,
		TTL:      DefaultTTL,
	}
}

// SRVRecord creates an SRV record for testing.
func SRVRecord(hostname, target string, port, priority, weight uint16) provider.Record {
	return provider.Record{
		Hostname: hostname,
		Type:     provider.RecordTypeSRV,
		Target:   target,
		TTL:      DefaultTTL,
		SRV: &provider.SRVData{
			Priority: priority,
			Weight:   weight,
			Port:     port,
		},
	}
}

// OwnershipRecord creates an ownership TXT record for the given hostname.
// instanceID can be empty for legacy format.
func OwnershipRecord(hostname, instanceID string) provider.Record {
	return provider.OwnershipRecord(hostname, DefaultTTL, instanceID, nil)
}

// OwnershipRecordWithMeta creates an ownership record with metadata.
func OwnershipRecordWithMeta(hostname, instanceID string, metadata map[string]string) provider.Record {
	return provider.OwnershipRecord(hostname, DefaultTTL, instanceID, metadata)
}

// RecordWithID creates a record with a provider-specific ID set.
func RecordWithID(r provider.Record, id string) provider.Record {
	r.ProviderID = id
	return r
}

// RecordWithMeta creates a record with metadata set.
func RecordWithMeta(r provider.Record, metadata map[string]string) provider.Record {
	r.Metadata = metadata
	return r
}

// --- Hostname builders ---

// Hostname creates a source.Hostname for testing.
func Hostname(name, src string) source.Hostname {
	return source.Hostname{
		Name:   name,
		Source: src,
	}
}

// HostnameWithRouter creates a source.Hostname with a router name.
func HostnameWithRouter(name, src, router string) source.Hostname {
	return source.Hostname{
		Name:   name,
		Source: src,
		Router: router,
	}
}

// HostnameWithHints creates a source.Hostname with record hints.
func HostnameWithHints(name, src string, hints *source.RecordHints) source.Hostname {
	return source.Hostname{
		Name:        name,
		Source:      src,
		RecordHints: hints,
	}
}
