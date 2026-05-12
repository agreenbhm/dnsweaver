package provider

// ProviderIdentity uniquely identifies the backend store that a provider
// writes to. Two ProviderInstances that share the same Identity AND
// RecordType resolve to the same physical record set; if both attempt to
// write the same hostname, they will race (each instance's cached view of
// the zone is stale relative to the other's writes, producing record flap —
// see issue #86).
//
// Identity is deliberately small and stable. Distinguishing fields:
//   - Type:     provider type ("cloudflare", "technitium", ...)
//   - Endpoint: API URL, server address, file path, or other backend locator
//   - Zone:     managed DNS zone (or "" when not applicable)
//
// Provider-specific tuning parameters (TTL, proxied state, auth credentials)
// are intentionally excluded — only the address of the data store is part of
// identity. Two instances with the same identity but different Targets are a
// misconfiguration (one is going to overwrite the other on every reconcile);
// the reconciler detects this and applies first-match-wins.
type ProviderIdentity struct {
	Type     string
	Endpoint string
	Zone     string
}

// Identifiable is implemented by providers that can report their backend
// identity. The reconciler uses this to detect overlapping write
// destinations (issue #88) and group matching providers so each distinct
// backend receives exactly one write per hostname per reconciliation.
//
// Providers that do not implement this interface fall back to the
// conservative identity {Type: provider.Type(), Endpoint: "", Zone: ""},
// which groups all instances of that type together — preserving the
// first-match-wins behavior from #86 for legacy providers.
//
// All built-in providers implement Identifiable. Implementing it is
// strongly recommended for any out-of-tree provider that supports multiple
// instances pointing at distinct backends (otherwise only one such instance
// will ever write).
type Identifiable interface {
	Identity() ProviderIdentity
}

// IdentityOf returns the identity for a Provider, falling back to a
// type-only identity for providers that do not implement Identifiable.
func IdentityOf(p Provider) ProviderIdentity {
	if id, ok := p.(Identifiable); ok {
		return id.Identity()
	}
	return ProviderIdentity{Type: p.Type()}
}
