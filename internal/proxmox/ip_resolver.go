package proxmox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// ResolveIP returns the primary IPv4 address for a Proxmox resource.
//
// For LXC containers (type "lxc"), it reads the net0 config field and parses
// the ip= component. For QEMU VMs (type "qemu"), it queries the qemu-guest-agent
// and returns the first non-loopback IPv4 address found on any interface.
//
// If the VM has no guest agent running, the function logs a warning and returns
// an empty string (no error), since this is a common condition in homelabs
// where not all VMs run the agent.
//
// Returns an empty string if no IP address can be determined.
func ResolveIP(ctx context.Context, client *Client, resource ClusterResource, logger *slog.Logger) (string, error) {
	switch resource.Type {
	case "lxc":
		return resolveLXCIP(ctx, client, resource)
	case "qemu":
		return resolveVMIP(ctx, client, resource, logger)
	default:
		return "", fmt.Errorf("proxmox: unsupported resource type %q", resource.Type)
	}
}

// resolveLXCIP reads the LXC container config and extracts the IP from the net0 field.
func resolveLXCIP(ctx context.Context, client *Client, resource ClusterResource) (string, error) {
	cfg, err := client.GetLXCConfig(ctx, resource.Node, resource.VMID)
	if err != nil {
		return "", fmt.Errorf("fetching config for LXC %d on %s: %w", resource.VMID, resource.Node, err)
	}

	if cfg.Net0 == "" {
		return "", nil
	}

	ip := parseLXCNet0IP(cfg.Net0)
	return ip, nil
}

// parseLXCNet0IP parses the ip= component from a Proxmox LXC net0 config value.
// Example input: "name=eth0,bridge=vmbr0,hwaddr=AA:BB:CC:DD:EE:FF,ip=192.0.2.50/24,ip6=auto"
// Returns the IP address without the CIDR prefix, or empty string if not found.
func parseLXCNet0IP(net0 string) string {
	for _, part := range strings.Split(net0, ",") {
		key, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		if key != "ip" {
			continue
		}
		if value == "dhcp" || value == "" {
			// DHCP — no static IP to return; caller can skip this resource.
			return ""
		}
		// Strip CIDR prefix length (e.g. "192.0.2.50/24" → "192.0.2.50").
		ip, _, _ := strings.Cut(value, "/")
		return ip
	}
	return ""
}

// resolveVMIP queries the qemu-guest-agent for the VM's network interfaces and
// returns the first non-loopback IPv4 address found.
func resolveVMIP(ctx context.Context, client *Client, resource ClusterResource, logger *slog.Logger) (string, error) {
	ifaces, err := client.GetVMAgentNetworks(ctx, resource.Node, resource.VMID)
	if err != nil {
		var agentErr *ErrAgentNotRunning
		if errors.As(err, &agentErr) {
			logger.Warn("qemu-guest-agent not running; skipping IP resolution",
				slog.String("node", resource.Node),
				slog.Int("vmid", resource.VMID),
				slog.String("name", resource.Name),
			)
			return "", nil
		}
		return "", fmt.Errorf("querying guest agent for VM %d on %s: %w", resource.VMID, resource.Node, err)
	}

	for _, iface := range ifaces {
		if isLoopback(iface.Name) {
			continue
		}
		for _, addr := range iface.IPAddresses {
			if addr.IPAddressType == "ipv4" && !isLoopbackIP(addr.IPAddress) {
				return addr.IPAddress, nil
			}
		}
	}

	return "", nil
}

// isLoopback returns true for known loopback interface names.
func isLoopback(name string) bool {
	return name == "lo" || name == "lo0"
}

// isLoopbackIP returns true for the IPv4 loopback address.
func isLoopbackIP(ip string) bool {
	return strings.HasPrefix(ip, "127.")
}
