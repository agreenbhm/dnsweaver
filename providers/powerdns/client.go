package powerdns

import (
	"fmt"
	"strconv"
	"strings"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// apiRecord is a single record entry within a PowerDNS rrset.
type apiRecord struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

// rrset is a PowerDNS resource record set: all records sharing a name+type.
// ChangeType is only set on PATCH requests ("REPLACE" or "DELETE").
type rrset struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	TTL        int         `json:"ttl,omitempty"`
	ChangeType string      `json:"changetype,omitempty"`
	Records    []apiRecord `json:"records"`
}

// zoneResponse is the subset of the PowerDNS zone object dnsweaver consumes.
type zoneResponse struct {
	Name   string  `json:"name"`
	RRsets []rrset `json:"rrsets"`
}

// patchRequest is the body of a PATCH .../zones/{zone} call.
type patchRequest struct {
	RRsets []rrset `json:"rrsets"`
}

// apiErrorBody is the PowerDNS error envelope ({"error": "..."}).
type apiErrorBody struct {
	Error string `json:"error"`
}

// canonicalize ensures a name ends in exactly one trailing dot, the canonical
// form PowerDNS uses for rrset names and CNAME/SRV targets.
func canonicalize(name string) string {
	if name == "" {
		return "."
	}
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

// stripDot removes a single trailing dot so the rest of dnsweaver sees bare names.
func stripDot(name string) string {
	return strings.TrimSuffix(name, ".")
}

// quoteTXT wraps TXT content in double quotes (PowerDNS's stored form),
// escaping embedded quotes and backslashes. Already-quoted input is returned
// unchanged so the function is idempotent.
func quoteTXT(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// unquoteTXT reverses quoteTXT. Unquoted input is returned unchanged.
func unquoteTXT(s string) string {
	if len(s) < 2 || !strings.HasPrefix(s, `"`) || !strings.HasSuffix(s, `"`) {
		return s
	}
	inner := s[1 : len(s)-1]
	var b strings.Builder
	escaped := false
	for _, r := range inner {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// encodeSRVContent builds PowerDNS SRV content: "<prio> <weight> <port> <target.>".
func encodeSRVContent(srv *provider.SRVData, target string) string {
	return fmt.Sprintf("%d %d %d %s", srv.Priority, srv.Weight, srv.Port, canonicalize(target))
}

// parseSRVContent parses PowerDNS SRV content into SRVData and the bare target.
func parseSRVContent(content string) (*provider.SRVData, string, error) {
	fields := strings.Fields(content)
	if len(fields) != 4 {
		return nil, "", fmt.Errorf("invalid SRV content %q: expected 4 fields", content)
	}
	prio, err := strconv.ParseUint(fields[0], 10, 16)
	if err != nil {
		return nil, "", fmt.Errorf("invalid SRV priority %q: %w", fields[0], err)
	}
	weight, err := strconv.ParseUint(fields[1], 10, 16)
	if err != nil {
		return nil, "", fmt.Errorf("invalid SRV weight %q: %w", fields[1], err)
	}
	port, err := strconv.ParseUint(fields[2], 10, 16)
	if err != nil {
		return nil, "", fmt.Errorf("invalid SRV port %q: %w", fields[2], err)
	}
	return &provider.SRVData{
		Priority: uint16(prio),
		Weight:   uint16(weight),
		Port:     uint16(port),
	}, stripDot(fields[3]), nil
}

// recordContent converts a provider.Record's value into PowerDNS rrset content.
func recordContent(rec provider.Record) (string, error) {
	switch rec.Type {
	case provider.RecordTypeA, provider.RecordTypeAAAA:
		return rec.Target, nil
	case provider.RecordTypeCNAME:
		return canonicalize(rec.Target), nil
	case provider.RecordTypeTXT:
		return quoteTXT(rec.Target), nil
	case provider.RecordTypeSRV:
		if rec.SRV == nil {
			return "", fmt.Errorf("SRV record requires SRV data")
		}
		return encodeSRVContent(rec.SRV, rec.Target), nil
	default:
		return "", fmt.Errorf("unsupported record type %q", rec.Type)
	}
}

// decodeContent converts PowerDNS rrset content back into provider.Record fields.
func decodeContent(rt provider.RecordType, content string) (target string, srv *provider.SRVData, err error) {
	switch rt {
	case provider.RecordTypeA, provider.RecordTypeAAAA:
		return content, nil, nil
	case provider.RecordTypeCNAME:
		return stripDot(content), nil, nil
	case provider.RecordTypeTXT:
		return unquoteTXT(content), nil, nil
	case provider.RecordTypeSRV:
		s, tgt, perr := parseSRVContent(content)
		if perr != nil {
			return "", nil, perr
		}
		return tgt, s, nil
	default:
		return "", nil, fmt.Errorf("unsupported record type %q", rt)
	}
}
