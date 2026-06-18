# OVHcloud

[OVHcloud](https://www.ovhcloud.com/) provides public DNS for domains registered or hosted with OVH. dnsweaver manages records through the OVH API for automated record management.

## Requirements

- An OVHcloud account with at least one DNS zone
- An OVH API application key, application secret, and consumer key with DNS edit permissions

## Basic Configuration

```yaml
environment:
  - DNSWEAVER_INSTANCES=ovh

  - DNSWEAVER_OVH_TYPE=ovh
  - DNSWEAVER_OVH_APPLICATION_KEY_FILE=/run/secrets/ovh_application_key
  - DNSWEAVER_OVH_APPLICATION_SECRET_FILE=/run/secrets/ovh_application_secret
  - DNSWEAVER_OVH_CONSUMER_KEY_FILE=/run/secrets/ovh_consumer_key
  - DNSWEAVER_OVH_ENDPOINT=ovh-eu
  - DNSWEAVER_OVH_ZONE=example.com
  - DNSWEAVER_OVH_RECORD_TYPE=A
  - DNSWEAVER_OVH_TARGET=203.0.113.100
  - DNSWEAVER_OVH_DOMAINS=*.example.com
```

## Configuration Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TYPE` | Yes | - | Must be `ovh` |
| `APPLICATION_KEY` | Yes | - | OVH application key |
| `APPLICATION_KEY_FILE` | Alt | - | Path to file containing the application key |
| `APPLICATION_SECRET` | Yes | - | OVH application secret |
| `APPLICATION_SECRET_FILE` | Alt | - | Path to file containing the application secret |
| `CONSUMER_KEY` | Yes | - | OVH consumer key |
| `CONSUMER_KEY_FILE` | Alt | - | Path to file containing the consumer key |
| `ENDPOINT` | No | `ovh-eu` | API region (see below) |
| `ZONE` | Yes | - | DNS zone name (e.g. `example.com`) |
| `RECORD_TYPE` | Yes | - | `A`, `AAAA`, `CNAME`, `SRV`, or `TXT` |
| `TARGET` | Yes | - | Record value |
| `DOMAINS` | Yes | - | Glob patterns to match |
| `EXCLUDE_DOMAINS` | No | - | Patterns to exclude |
| `TTL` | No | `3600` | TTL in seconds (minimum `60`, or `0` for the zone default) |

All three credentials support the `_FILE` suffix for Docker/Kubernetes secrets.

### API Regions

OVH operates several API endpoints. Set `ENDPOINT` to the region that matches your account:

| `ENDPOINT` | API base URL |
|------------|--------------|
| `ovh-eu` (default) | `https://eu.api.ovh.com/1.0` |
| `ovh-ca` | `https://ca.api.ovh.com/1.0` |
| `ovh-us` | `https://api.us.ovhcloud.com/1.0` |
| `kimsufi-eu` | `https://eu.api.kimsufi.com/1.0` |
| `kimsufi-ca` | `https://ca.api.kimsufi.com/1.0` |
| `soyoustart-eu` | `https://eu.api.soyoustart.com/1.0` |
| `soyoustart-ca` | `https://ca.api.soyoustart.com/1.0` |

## Creating API Credentials

dnsweaver needs an OVH token with these **five** access rules (replace `example.com` with your zone):

```
GET    /domain/zone/
GET    /domain/zone/example.com/*
PUT    /domain/zone/example.com/*
POST   /domain/zone/example.com/*
DELETE /domain/zone/example.com/*
```

!!! danger "Every rule path must start with a leading `/`"
    This is the single most common cause of a `403 This call has not been
    granted` at startup. OVH matches the **request** path with a leading slash
    (`/domain/zone/example.com/soa`), so a rule stored **without** the leading
    slash (`domain/zone/example.com/*`) matches nothing.

    The web token page (<https://api.ovh.com/createToken/>) has been observed to
    **silently strip the leading slash from wildcard rules**, leaving you with an
    unusable `domain/zone/example.com/*` even though you typed it correctly.
    **Create the token via the API instead** (below) — it stores the
    `accessRules` paths verbatim. After creating any token, verify the stored
    rules with `GET /auth/currentCredential` and confirm each `path` begins with
    `/`.

### Recommended: create the token via the API

This stores the rules exactly as written and avoids the web UI's slash-stripping.
Send your **Application Key** in the header (no consumer key or signature needed
for this call):

```bash
curl -X POST https://eu.api.ovh.com/1.0/auth/credential \
  -H "X-Ovh-Application: $OVH_APPLICATION_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "accessRules": [
      {"method": "GET",    "path": "/domain/zone/"},
      {"method": "GET",    "path": "/domain/zone/example.com/*"},
      {"method": "PUT",    "path": "/domain/zone/example.com/*"},
      {"method": "POST",   "path": "/domain/zone/example.com/*"},
      {"method": "DELETE", "path": "/domain/zone/example.com/*"}
    ],
    "redirection": "https://eu.api.ovh.com/"
  }'
```

The response contains a `consumerKey` and a `validationUrl`. Open the
`validationUrl`, sign in, set **Validity** to *Unlimited*, and validate. Use the
returned `consumerKey` together with the same Application Key and Application
Secret. Store each value as a secret and reference it via the `_FILE` variables.

!!! tip "Manage every zone with one token"
    To let a single token manage any zone in the account, use
    `GET/PUT/POST/DELETE /domain/zone/*` (with the leading slash) instead of the
    per-zone paths above.

The full set of API calls dnsweaver makes, all under `/domain/zone/<zone>/`:

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/soa` | Connectivity / credential check at startup |
| `GET` | `/record` and `/record/{id}` | List and read records |
| `POST` | `/record` | Create a record |
| `PUT` | `/record/{id}` | Update a record |
| `DELETE` | `/record/{id}` | Delete a record |
| `POST` | `/refresh` | Apply changes to the zone |

## How It Works

OVH stores records relative to the zone (subdomains), so dnsweaver converts hostnames automatically:

- `app.example.com` (zone `example.com`) → subdomain `app`
- `example.com` → the zone apex (empty subdomain)

After every create, update, or delete, dnsweaver issues a **zone refresh** so changes propagate to OVH's DNS servers without manual intervention.

## Record Types

### A / AAAA Records

```yaml
- DNSWEAVER_OVH_RECORD_TYPE=A
- DNSWEAVER_OVH_TARGET=203.0.113.100
```

### CNAME Records

```yaml
- DNSWEAVER_OVH_RECORD_TYPE=CNAME
- DNSWEAVER_OVH_TARGET=proxy.example.com
```

### SRV Records

SRV records use the `_service._proto.name` hostname convention and are encoded by OVH as `priority weight port target`. dnsweaver handles the encoding and parsing for you.

## TLS Configuration

OVH's public API uses publicly-trusted certificates, so the defaults work out of the box. The shared TLS surface is still available for environments that proxy outbound traffic through an inspecting middlebox with its own CA:

| Env key | Purpose |
|---------|---------|
| `DNSWEAVER_OVH_TLS_CA_FILE` | Trust an additional CA bundle (PEM) |
| `DNSWEAVER_OVH_TLS_CERT_FILE` / `_TLS_KEY_FILE` | Present a client certificate (mTLS) |
| `DNSWEAVER_OVH_TLS_SERVER_NAME` | Override SNI / hostname verification |
| `DNSWEAVER_OVH_TLS_SKIP_VERIFY` | Disable verification (development only — **never** in production) |
| `DNSWEAVER_OVH_TLS_MIN_VERSION` | `1.2` (default) or `1.3` |

## Troubleshooting

### Authentication / Signature Errors

OVH signs requests with the server clock. If you see `INVALID_SIGNATURE` or timestamp errors, ensure the host running dnsweaver has reasonably accurate time (dnsweaver syncs to OVH's server time automatically, but a wildly skewed clock can still cause issues).

### Call Not Granted

A `This call has not been granted` error means the consumer key's access rules don't authorize the call. By far the most common cause is a **rule path stored without its leading `/`**: OVH matches the request path as `/domain/zone/<zone>/soa`, so a rule saved as `domain/zone/<zone>/*` (no leading slash) matches nothing. The web token page can strip the slash from wildcard rules even when you type it correctly.

To confirm, inspect the key's actual rules:

```bash
# (signed request) GET /auth/currentCredential
```

Each `path` in the returned `rules` must begin with `/`. If any wildcard rule shows up as `domain/zone/...` without the leading slash, recreate the token via the API as described in [Creating API Credentials](#creating-api-credentials), which preserves the paths verbatim.

This error is specifically about access rules — it is **not** a region or signature problem (those surface as `INVALID_SIGNATURE` or `This application key is invalid`). If `GET /domain/zone` (the zone list) succeeds but `GET /domain/zone/<zone>/soa` returns this error, the leading-slash issue is the likely culprit.

### Wrong Region

If credentials created in one region fail, confirm `ENDPOINT` matches the region where the token was created (EU credentials do not work against the CA endpoint and vice versa).
