# PowerDNS

Manage DNS records in a [PowerDNS Authoritative Server](https://doc.powerdns.com/authoritative/)
zone through its native HTTP API (`/api/v1`, `X-API-Key` authentication).

!!! note "PowerDNS via the HTTP API vs. RFC 2136"
    dnsweaver can drive PowerDNS two ways. This `powerdns` provider uses the
    **native Authoritative HTTP API** — enable it with `api=yes` and
    `api-key=...` in `pdns.conf`. If you instead run PowerDNS with DNS UPDATE
    (TSIG) enabled, use the [RFC 2136](rfc2136.md) provider. Prefer this
    provider when the HTTP API is available: clearer errors and no TSIG key
    management.

## Record Types

| Type | Supported |
|------|-----------|
| A | ✅ |
| AAAA | ✅ |
| CNAME | ✅ |
| SRV | ✅ |
| TXT | ✅ (used for ownership tracking) |

## Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DNSWEAVER_{NAME}_TYPE` | Yes | — | Must be `powerdns`. |
| `DNSWEAVER_{NAME}_URL` | Yes | — | Server base URL, e.g. `http://ns1.example.com:8081`. `/api/v1` is appended automatically. |
| `DNSWEAVER_{NAME}_API_KEY` | Yes | — | API key sent as the `X-API-Key` header. Supports the `_FILE` suffix for Docker/Kubernetes secrets. |
| `DNSWEAVER_{NAME}_ZONE` | Yes | — | Managed zone name, e.g. `example.com`. The zone **must already exist** in PowerDNS. |
| `DNSWEAVER_{NAME}_SERVER_ID` | No | `localhost` | PowerDNS server id segment in the API path. |
| `DNSWEAVER_{NAME}_TTL` | No | `300` | Default TTL for created records. |

TLS hardening (custom CA, mTLS client certs, SNI override, minimum TLS version,
or skipping verification) is configured with the shared `DNSWEAVER_{NAME}_TLS_*`
variables — see [Environment Variables](../configuration/environment.md).

## Example

```yaml
services:
  dnsweaver:
    image: maxamill/dnsweaver:latest
    environment:
      DNSWEAVER_INSTANCES: pdns
      DNSWEAVER_PDNS_TYPE: powerdns
      DNSWEAVER_PDNS_URL: http://ns1.example.com:8081
      DNSWEAVER_PDNS_API_KEY_FILE: /run/secrets/pdns_api_key
      DNSWEAVER_PDNS_ZONE: example.com
      DNSWEAVER_PDNS_TTL: "300"
```

## Enabling the PowerDNS API

In `pdns.conf` on the authoritative server:

```ini
api=yes
api-key=changeme-to-a-strong-key
webserver=yes
webserver-address=0.0.0.0
webserver-port=8081
webserver-allow-from=0.0.0.0/0   # restrict to dnsweaver's source network
```

Restart `pdns` and confirm the zone exists:

```bash
curl -H 'X-API-Key: changeme-to-a-strong-key' \
  http://ns1.example.com:8081/api/v1/servers/localhost/zones/example.com.
```

## Behavior Notes

- **Zone must pre-exist.** dnsweaver never creates or deletes zones; it only
  manages records within the configured zone. If the zone is missing, startup
  fails with an actionable error.
- **rrset semantics.** PowerDNS groups records by name+type. dnsweaver merges
  individual records into the existing rrset, so multiple values at the same
  name (e.g. round-robin A records) are preserved.
- **Ownership tracking.** TXT ownership records (`_dnsweaver.<host>`) are
  supported, enabling multi-instance safety.
