# dnsmasq

dnsmasq is a lightweight DNS/DHCP server commonly used in routers and containers. dnsweaver manages dnsmasq through configuration files.

## Requirements

- Write access to dnsmasq's configuration directory
- Ability to signal dnsmasq to reload (or dnsmasq configured to watch files)

## Basic Configuration

```yaml
environment:
  - DNSWEAVER_INSTANCES=dnsmasq

  - DNSWEAVER_DNSMASQ_TYPE=dnsmasq
  - DNSWEAVER_DNSMASQ_CONFIG_DIR=/etc/dnsmasq.d
  - DNSWEAVER_DNSMASQ_RECORD_TYPE=A
  - DNSWEAVER_DNSMASQ_TARGET=10.0.0.100
  - DNSWEAVER_DNSMASQ_DOMAINS=*.home.example.com
volumes:
  - /path/to/dnsmasq.d:/etc/dnsmasq.d
```

## Configuration Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TYPE` | Yes | - | Must be `dnsmasq` |
| `CONFIG_DIR` | No | `/etc/dnsmasq.d` | Path to dnsmasq config directory |
| `CONFIG_FILE` | No | `dnsweaver.conf` | Filename for managed records |
| `RELOAD_COMMAND` | No | `systemctl reload dnsmasq` | Command to reload dnsmasq |
| `ZONE` | No | - | DNS zone for record filtering |
| `TTL` | No | `300` | Record TTL in seconds |
| `RECORD_TYPE` | Yes | - | `A`, `AAAA`, or `CNAME` |
| `TARGET` | Yes | - | Record value |
| `DOMAINS` | Yes | - | Glob patterns to match |
| `EXCLUDE_DOMAINS` | No | - | Patterns to exclude |

### SSH Configuration

When managing a remote dnsmasq instance, add these variables to enable SSH mode:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SSH_HOST` | Yes* | - | SSH server hostname or IP |
| `SSH_PORT` | No | `22` | SSH server port |
| `SSH_USER` | Yes* | - | SSH username |
| `SSH_KEY_FILE` | No† | - | Path to SSH private key file |
| `SSH_PASSWORD` | No† | - | SSH password |

\* Required when SSH mode is enabled (any SSH variable is set).

† At least one authentication method is required: `SSH_KEY_FILE` or `SSH_PASSWORD`. Key-based authentication is strongly recommended.

`SSH_KEY_FILE` and `SSH_PASSWORD` support the `_FILE` suffix for [Docker secrets](../configuration/secrets.md). For example, use `SSH_KEY_FILE_FILE` to read the private key path from a secret, or `SSH_PASSWORD_FILE` to read the password from a secret.

## How It Works

dnsweaver creates a configuration file in the dnsmasq directory:

```
# /etc/dnsmasq.d/dnsweaver.conf (managed by dnsweaver)
address=/app.home.example.com/10.0.0.100
address=/web.home.example.com/10.0.0.100
cname=alias.home.example.com,target.home.example.com
```

## Record Types

### A Records

```yaml
- DNSWEAVER_DNSMASQ_RECORD_TYPE=A
- DNSWEAVER_DNSMASQ_TARGET=10.0.0.100
```

Produces:
```
address=/hostname.example.com/10.0.0.100
```

### AAAA Records

```yaml
- DNSWEAVER_DNSMASQ_RECORD_TYPE=AAAA
- DNSWEAVER_DNSMASQ_TARGET=2001:db8::1
```

Produces:
```
address=/hostname.example.com/2001:db8::1
```

### CNAME Records

```yaml
- DNSWEAVER_DNSMASQ_RECORD_TYPE=CNAME
- DNSWEAVER_DNSMASQ_TARGET=proxy.example.com
```

Produces:
```
cname=hostname.example.com,proxy.example.com
```

## Reloading dnsmasq

After file changes, dnsmasq needs to reload its configuration. Options:

### 1. Automatic (with inotify)

Some dnsmasq versions support watching for file changes. No reload command needed.

### 2. SIGHUP

Send HUP signal to reload:

```yaml
- DNSWEAVER_DNSMASQ_RELOAD_COMMAND=pkill -HUP dnsmasq
```

### 3. Restart Service

For systemd-managed dnsmasq:

```yaml
- DNSWEAVER_DNSMASQ_RELOAD_COMMAND=systemctl reload dnsmasq
```

!!! note
    The reload command runs inside the dnsweaver container. For remote dnsmasq, you may need a different approach (SSH, HTTP trigger, etc.).

## SSH Remote Management

dnsweaver can manage dnsmasq instances running on remote hosts via SSH. When SSH mode is enabled, dnsweaver uses SFTP to write configuration files and SSH exec to run reload commands on the remote system — no shared volumes or local mounts required.

### When to Use SSH Mode

| Scenario | SSH Mode? | Notes |
|----------|-----------|-------|
| dnsmasq in a sidecar/shared Docker volume | No | Use volume mounts |
| dnsmasq on a remote server or VM | **Yes** | SSH manages files remotely |
| dnsmasq on a router (OpenWrt, DD-WRT) | **Yes** | SSH is the standard access method |
| dnsmasq in a different Docker host | **Yes** | No shared filesystem |

### Basic SSH Configuration

```yaml
environment:
  - DNSWEAVER_INSTANCES=router

  - DNSWEAVER_ROUTER_TYPE=dnsmasq
  - DNSWEAVER_ROUTER_CONFIG_DIR=/tmp/dnsmasq.d
  - DNSWEAVER_ROUTER_RECORD_TYPE=A
  - DNSWEAVER_ROUTER_TARGET=10.0.0.100
  - DNSWEAVER_ROUTER_DOMAINS=*.home.example.com
  - DNSWEAVER_ROUTER_RELOAD_COMMAND=killall -HUP dnsmasq

  # SSH connection
  - DNSWEAVER_ROUTER_SSH_HOST=192.168.1.1
  - DNSWEAVER_ROUTER_SSH_PORT=22
  - DNSWEAVER_ROUTER_SSH_USER=root
  - DNSWEAVER_ROUTER_SSH_KEY_FILE=/run/secrets/router_ssh_key
```

!!! note
    When SSH mode is enabled, `CONFIG_DIR` refers to a path **on the remote host**, not inside the dnsweaver container. `RELOAD_COMMAND` also executes on the remote host via SSH exec.

### Authentication Methods

SSH supports three authentication methods, in order of preference:

#### 1. Key File (Recommended)

Mount a private key file into the container:

```yaml
environment:
  - DNSWEAVER_ROUTER_SSH_KEY_FILE=/ssh/id_ed25519
volumes:
  - ./ssh_keys/router_key:/ssh/id_ed25519:ro
```

Or use Docker secrets:

```yaml
environment:
  - DNSWEAVER_ROUTER_SSH_KEY_FILE_FILE=/run/secrets/router_ssh_key
secrets:
  - router_ssh_key
```

#### 2. Password Authentication

Use password auth as a fallback (not recommended for production):

```yaml
environment:
  - DNSWEAVER_ROUTER_SSH_PASSWORD_FILE=/run/secrets/router_password
secrets:
  - router_password
```

!!! note
    The dnsmasq provider uses `InsecureIgnoreHostKey` for SSH connections. This is suitable for trusted internal networks. Future versions may add configurable host key verification.

### Docker Compose with SSH

Complete example managing a remote router:

```yaml
services:
  dnsweaver:
    image: maxamill/dnsweaver:latest
    environment:
      - DNSWEAVER_INSTANCES=router
      - DNSWEAVER_ROUTER_TYPE=dnsmasq
      - DNSWEAVER_ROUTER_CONFIG_DIR=/tmp/dnsmasq.d
      - DNSWEAVER_ROUTER_CONFIG_FILE=dnsweaver.conf
      - DNSWEAVER_ROUTER_RECORD_TYPE=A
      - DNSWEAVER_ROUTER_TARGET=10.0.0.100
      - DNSWEAVER_ROUTER_DOMAINS=*.home.example.com
      - DNSWEAVER_ROUTER_RELOAD_COMMAND=killall -HUP dnsmasq
      - DNSWEAVER_ROUTER_SSH_HOST=192.168.1.1
      - DNSWEAVER_ROUTER_SSH_USER=root
      - DNSWEAVER_ROUTER_SSH_KEY_FILE_FILE=/run/secrets/router_ssh_key
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    secrets:
      - router_ssh_key

secrets:
  router_ssh_key:
    file: ./ssh_keys/router_id_ed25519
```

## Docker Deployment

When running dnsweaver with dnsmasq in Docker:

```yaml
services:
  dnsweaver:
    image: maxamill/dnsweaver:latest
    environment:
      - DNSWEAVER_DNSMASQ_CONFIG_DIR=/etc/dnsmasq.d
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - dnsmasq_config:/etc/dnsmasq.d

  dnsmasq:
    image: jpillora/dnsmasq:latest
    volumes:
      - dnsmasq_config:/etc/dnsmasq.d
    ports:
      - "53:53/udp"

volumes:
  dnsmasq_config:
```

## Router Integration

For routers running dnsmasq (OpenWrt, DD-WRT, etc.), SSH mode is the recommended approach:

### SSH Mode (Recommended)

Use SSH to manage the router's dnsmasq configuration directly:

```yaml
environment:
  - DNSWEAVER_ROUTER_TYPE=dnsmasq
  - DNSWEAVER_ROUTER_CONFIG_DIR=/tmp/dnsmasq.d
  - DNSWEAVER_ROUTER_RELOAD_COMMAND=killall -HUP dnsmasq
  - DNSWEAVER_ROUTER_SSH_HOST=192.168.1.1
  - DNSWEAVER_ROUTER_SSH_USER=root
  - DNSWEAVER_ROUTER_SSH_KEY_FILE=/ssh/router_key
```

See [SSH Remote Management](#ssh-remote-management) for complete configuration details including authentication and secrets.

### Volume Mount (Alternative)

If the router's filesystem is available via NFS or CIFS:

1. Mount the router's dnsmasq config directory
2. Configure dnsweaver to write to that mount
3. Set up a reload mechanism (SSH command, webhook, etc.)

## Troubleshooting

### Records Not Updating

Check the managed config file:

```bash
cat /etc/dnsmasq.d/dnsweaver.conf
```

For SSH mode, check on the **remote host**:

```bash
ssh root@192.168.1.1 "cat /tmp/dnsmasq.d/dnsweaver.conf"
```

### Reload Not Working

Verify the reload command:

```bash
docker exec dnsweaver /bin/sh -c "$RELOAD_COMMAND"
```

For SSH mode, the reload command runs on the remote host. Verify that the command works when run manually via SSH.

### Permission Denied

Ensure dnsweaver can write to the config directory:

```bash
docker exec dnsweaver touch /etc/dnsmasq.d/test.conf
```

For SSH mode, ensure the SSH user has write access to `CONFIG_DIR` on the remote host.

### SSH Connection Refused

Verify SSH connectivity from the dnsweaver container:

- Check that `SSH_HOST` and `SSH_PORT` are correct
- Ensure the SSH service is running on the remote host
- Verify firewall rules allow the connection
- Check logs for authentication errors (wrong key, wrong user)

### SSH Authentication Failed

- **Key file:** Verify the key file is mounted correctly and readable (`chmod 600`)
- **Password:** Confirm the remote host allows password authentication

### SSH Timeout

If SSH connections are timing out, verify that the remote host is reachable and the SSH service is running. Check firewall rules and network connectivity.
