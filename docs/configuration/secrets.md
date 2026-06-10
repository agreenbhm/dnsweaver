# Secrets Management

dnsweaver supports multiple methods for secure credential management across Docker and Kubernetes. Any environment variable can use the `_FILE` suffix to read its value from a file — this works with Docker secrets, Kubernetes secret volume mounts, or any file-based secret injection.

## How It Works

The `_FILE` suffix pattern works identically on Docker and Kubernetes — dnsweaver reads the file contents at startup and uses them as the variable value.

Instead of passing a secret directly:

```yaml
environment:
  - DNSWEAVER_INTERNAL_DNS_TOKEN=my-secret-token  # ❌ Exposed in environment
```

Use the `_FILE` suffix to read from a secrets file:

```yaml
environment:
  - DNSWEAVER_INTERNAL_DNS_TOKEN_FILE=/run/secrets/dns_token  # ✅ Secure
secrets:
  - dns_token
```

## Docker Compose Example

```yaml
services:
  dnsweaver:
    image: maxamill/dnsweaver:latest
    environment:
      - DNSWEAVER_INSTANCES=internal-dns,cloudflare

      # Technitium with secret
      - DNSWEAVER_INTERNAL_DNS_TYPE=technitium
      - DNSWEAVER_INTERNAL_DNS_URL=http://dns:5380
      - DNSWEAVER_INTERNAL_DNS_TOKEN_FILE=/run/secrets/technitium_token
      - DNSWEAVER_INTERNAL_DNS_ZONE=home.example.com
      - DNSWEAVER_INTERNAL_DNS_RECORD_TYPE=A
      - DNSWEAVER_INTERNAL_DNS_TARGET=192.0.2.100
      - DNSWEAVER_INTERNAL_DNS_DOMAINS=*.home.example.com

      # Cloudflare with secret
      - DNSWEAVER_CLOUDFLARE_TYPE=cloudflare
      - DNSWEAVER_CLOUDFLARE_TOKEN_FILE=/run/secrets/cloudflare_token
      - DNSWEAVER_CLOUDFLARE_ZONE=example.com
      - DNSWEAVER_CLOUDFLARE_RECORD_TYPE=CNAME
      - DNSWEAVER_CLOUDFLARE_TARGET=proxy.example.com
      - DNSWEAVER_CLOUDFLARE_DOMAINS=*.example.com
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    secrets:
      - technitium_token
      - cloudflare_token

secrets:
  technitium_token:
    external: true
  cloudflare_token:
    external: true
```

## Docker Swarm Example

In Swarm mode, create secrets with `docker secret create`:

```bash
# Create secrets
echo "your-technitium-token" | docker secret create technitium_token -
echo "your-cloudflare-token" | docker secret create cloudflare_token -
```

Then reference them in your stack file exactly as shown above.

## Supported Variables

Any environment variable that accepts sensitive data supports the `_FILE` suffix:

### Provider Credentials

| Variable | File Suffix |
|----------|-------------|
| `DNSWEAVER_{NAME}_TOKEN` | `DNSWEAVER_{NAME}_TOKEN_FILE` |
| `DNSWEAVER_{NAME}_PASSWORD` | `DNSWEAVER_{NAME}_PASSWORD_FILE` |
| `DNSWEAVER_{NAME}_AUTH_TOKEN` | `DNSWEAVER_{NAME}_AUTH_TOKEN_FILE` |

### SSH Authentication

Providers that support SSH remote management (e.g., [dnsmasq](../providers/dnsmasq.md#ssh-remote-management)) use these variables:

| Variable | File Suffix | Description |
|----------|-------------|-------------|
| `DNSWEAVER_{NAME}_SSH_KEY_FILE` | `DNSWEAVER_{NAME}_SSH_KEY_FILE_FILE` | Path to SSH private key |
| `DNSWEAVER_{NAME}_SSH_PASSWORD` | `DNSWEAVER_{NAME}_SSH_PASSWORD_FILE` | SSH password |
| `DNSWEAVER_{NAME}_SSH_KNOWN_HOSTS_FILE` | `DNSWEAVER_{NAME}_SSH_KNOWN_HOSTS_FILE_FILE` | Path to OpenSSH `known_hosts` file |

### SSH Key via Docker Secret

To pass an SSH private key as a Docker secret:

```yaml
services:
  dnsweaver:
    environment:
      # Read the SSH key path from a Docker secret
      - DNSWEAVER_ROUTER_SSH_KEY_FILE_FILE=/run/secrets/router_ssh_key
    secrets:
      - router_ssh_key

secrets:
  router_ssh_key:
    file: ./ssh_keys/router_id_ed25519
```

## Secret File Format

Secret files should contain only the secret value, with optional trailing newline:

```
my-secret-token-value
```

!!! warning
    Do not include variable names, quotes, or other formatting in secret files. Just the raw value.

## Kubernetes Secrets

On Kubernetes, you have two approaches for injecting secrets:

### Option 1: Environment Variable from Secret (Recommended)

Reference Kubernetes Secrets directly as environment variables:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dnsweaver
spec:
  template:
    spec:
      containers:
        - name: dnsweaver
          env:
            - name: DNSWEAVER_INTERNAL_DNS_TOKEN
              valueFrom:
                secretKeyRef:
                  name: dnsweaver-credentials
                  key: technitium-token
            - name: DNSWEAVER_CLOUDFLARE_TOKEN
              valueFrom:
                secretKeyRef:
                  name: dnsweaver-credentials
                  key: cloudflare-token
```

Create the Secret:

```bash
kubectl create secret generic dnsweaver-credentials \
  --namespace dnsweaver \
  --from-literal=technitium-token=your-technitium-token \
  --from-literal=cloudflare-token=your-cloudflare-token
```

### Option 2: Volume Mount with `_FILE` Suffix

Mount Secrets as files and use the `_FILE` suffix:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dnsweaver
spec:
  template:
    spec:
      containers:
        - name: dnsweaver
          env:
            - name: DNSWEAVER_INTERNAL_DNS_TOKEN_FILE
              value: /etc/dnsweaver/secrets/technitium-token
          volumeMounts:
            - name: credentials
              mountPath: /etc/dnsweaver/secrets
              readOnly: true
      volumes:
        - name: credentials
          secret:
            secretName: dnsweaver-credentials
```

### External Secret Operators

dnsweaver works with any Kubernetes secrets operator that produces standard Secrets, including:

- [External Secrets Operator](https://external-secrets.io/) (Vault, AWS SSM, etc.)
- [Sealed Secrets](https://sealed-secrets.netlify.app/)
- [Infisical Secrets Operator](https://infisical.com/)

Simply reference the generated Secret name in your dnsweaver configuration.

!!! tip "Helm chart secrets"
    The dnsweaver Helm chart has built-in support for creating Secrets from values and referencing existing Secrets. See the [Kubernetes deployment guide](../deployment/kubernetes.md) for details.
