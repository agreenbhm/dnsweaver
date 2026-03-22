package testutil

// TechnitiumSuccess wraps a response payload in the Technitium success envelope.
func TechnitiumSuccess(response any) map[string]any {
	return map[string]any{
		"status":   "ok",
		"response": response,
	}
}

// TechnitiumError returns a Technitium API error response.
func TechnitiumError(message string) map[string]any {
	return map[string]any{
		"status":       "error",
		"errorMessage": message,
	}
}

// TechnitiumZoneInfo returns a zone info block for Technitium responses.
func TechnitiumZoneInfo(zone string) map[string]any {
	return map[string]any{
		"name":     zone,
		"type":     "Primary",
		"disabled": false,
	}
}

// TechnitiumRecord returns a single Technitium API record.
// recordType should be "A", "AAAA", "CNAME", "TXT", or "SRV".
func TechnitiumRecord(name, recordType string, ttl int, rData map[string]any) map[string]any {
	return map[string]any{
		"name":     name,
		"type":     recordType,
		"ttl":      ttl,
		"disabled": false,
		"rData":    rData,
	}
}

// TechnitiumListResponse returns a full zone record listing response.
func TechnitiumListResponse(zone string, records ...map[string]any) map[string]any {
	return TechnitiumSuccess(map[string]any{
		"zone":    TechnitiumZoneInfo(zone),
		"records": records,
	})
}

// --- Cloudflare response builders ---

// CloudflareSuccess wraps a result in the Cloudflare success envelope.
func CloudflareSuccess(result any) map[string]any {
	return map[string]any{
		"success":  true,
		"errors":   []any{},
		"messages": []any{},
		"result":   result,
	}
}

// CloudflareError returns a Cloudflare API error response.
func CloudflareError(code int, message string) map[string]any {
	return map[string]any{
		"success": false,
		"errors": []map[string]any{
			{"code": code, "message": message},
		},
		"messages": []any{},
		"result":   nil,
	}
}

// CloudflareRecord returns a single Cloudflare DNS record object.
func CloudflareRecord(id, recordType, name, content string, ttl int, proxied bool) map[string]any {
	return map[string]any{
		"id":      id,
		"type":    recordType,
		"name":    name,
		"content": content,
		"ttl":     ttl,
		"proxied": proxied,
	}
}

// --- Pi-hole v5 response builders ---

// PiholeV5DNSList returns a Pi-hole v5 custom DNS list response.
// Each entry is [IP, hostname].
func PiholeV5DNSList(entries ...[]string) map[string]any {
	data := make([][]string, len(entries))
	copy(data, entries)
	return map[string]any{"data": data}
}

// PiholeV5CNAMEList returns a Pi-hole v5 CNAME list response.
// Each entry is [alias, target].
func PiholeV5CNAMEList(entries ...[]string) map[string]any {
	data := make([][]string, len(entries))
	copy(data, entries)
	return map[string]any{"data": data}
}

// --- Pi-hole v6 response builders ---

// PiholeV6AuthSuccess returns a Pi-hole v6 authentication success response.
func PiholeV6AuthSuccess(sid string) map[string]any {
	return map[string]any{
		"session": map[string]any{
			"valid":    true,
			"sid":      sid,
			"validity": 300,
		},
	}
}

// PiholeV6DNSConfig returns a Pi-hole v6 DNS config response.
func PiholeV6DNSConfig(hosts, cnameRecords []string) map[string]any {
	return map[string]any{
		"config": map[string]any{
			"dns": map[string]any{
				"hosts":        hosts,
				"cnameRecords": cnameRecords,
			},
		},
	}
}

// --- Webhook response builders ---

// WebhookRecord returns a single webhook RecordResponse.
func WebhookRecord(hostname, recordType, value string, ttl int) map[string]any {
	return map[string]any{
		"hostname": hostname,
		"type":     recordType,
		"value":    value,
		"ttl":      ttl,
	}
}

// WebhookRecordWithID returns a webhook RecordResponse with an ID.
func WebhookRecordWithID(id, hostname, recordType, value string, ttl int) map[string]any {
	return map[string]any{
		"id":       id,
		"hostname": hostname,
		"type":     recordType,
		"value":    value,
		"ttl":      ttl,
	}
}

// WebhookError returns a webhook error response.
func WebhookError(errMsg, message string) map[string]any {
	return map[string]any{
		"error":   errMsg,
		"message": message,
	}
}
