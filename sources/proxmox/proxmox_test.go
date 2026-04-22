package proxmox

import (
	"context"
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/workload"
)

func TestExtract_SkipsNonProxmox(t *testing.T) {
	src := New()
	w := workload.Workload{
		Platform: workload.PlatformDocker,
		Name:     "some-container",
		Metadata: map[string]string{"ip": "10.1.20.5"},
	}
	hostnames, err := src.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hostnames) != 0 {
		t.Errorf("expected no hostnames for non-proxmox workload, got %d", len(hostnames))
	}
}

func TestExtract_SkipsNoIP(t *testing.T) {
	src := New(WithDomain("home.example.com"))
	w := workload.Workload{
		Platform: workload.PlatformProxmox,
		Name:     "webserver",
		Metadata: map[string]string{"node": "pve-00", "vmid": "100"},
	}
	hostnames, err := src.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hostnames) != 0 {
		t.Errorf("expected no hostnames when IP is missing, got %d", len(hostnames))
	}
}

func TestExtract_NoDomainPlainName_Skips(t *testing.T) {
	src := New() // no domain configured
	w := workload.Workload{
		Platform: workload.PlatformProxmox,
		Name:     "webserver",
		Metadata: map[string]string{"ip": "10.1.20.5", "node": "pve-00", "vmid": "100"},
	}
	hostnames, err := src.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hostnames) != 0 {
		t.Errorf("expected no hostnames when no domain and name is not FQDN, got %d", len(hostnames))
	}
}

func TestExtract_WithDomain(t *testing.T) {
	src := New(WithDomain("home.example.com"))
	w := workload.Workload{
		Platform: workload.PlatformProxmox,
		Name:     "webserver",
		Metadata: map[string]string{"ip": "10.1.20.5", "node": "pve-00", "vmid": "100"},
	}
	hostnames, err := src.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hostnames) != 1 {
		t.Fatalf("expected 1 hostname, got %d", len(hostnames))
	}
	h := hostnames[0]
	if h.Name != "webserver.home.example.com" {
		t.Errorf("expected Name=%q, got %q", "webserver.home.example.com", h.Name)
	}
	if h.Source != "proxmox" {
		t.Errorf("expected Source=%q, got %q", "proxmox", h.Source)
	}
	if h.Router != "pve-00/100" {
		t.Errorf("expected Router=%q, got %q", "pve-00/100", h.Router)
	}
	if h.RecordHints == nil {
		t.Fatal("expected RecordHints to be set")
	}
	if h.RecordHints.Type != "A" {
		t.Errorf("expected RecordHints.Type=%q, got %q", "A", h.RecordHints.Type)
	}
	if h.RecordHints.Target != "10.1.20.5" {
		t.Errorf("expected RecordHints.Target=%q, got %q", "10.1.20.5", h.RecordHints.Target)
	}
}

func TestExtract_NameIsFQDN(t *testing.T) {
	src := New() // no domain — VM name is already FQDN
	w := workload.Workload{
		Platform: workload.PlatformProxmox,
		Name:     "webserver.home.example.com",
		Metadata: map[string]string{"ip": "10.1.20.5", "node": "pve-00", "vmid": "100"},
	}
	hostnames, err := src.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hostnames) != 1 {
		t.Fatalf("expected 1 hostname, got %d", len(hostnames))
	}
	if hostnames[0].Name != "webserver.home.example.com" {
		t.Errorf("expected Name=%q, got %q", "webserver.home.example.com", hostnames[0].Name)
	}
}

func TestExtract_DomainWithLeadingDot(t *testing.T) {
	src := New(WithDomain(".home.example.com")) // leading dot should be stripped
	w := workload.Workload{
		Platform: workload.PlatformProxmox,
		Name:     "db",
		Metadata: map[string]string{"ip": "10.1.20.6", "node": "pve-01", "vmid": "101"},
	}
	hostnames, err := src.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hostnames) != 1 {
		t.Fatalf("expected 1 hostname, got %d", len(hostnames))
	}
	if hostnames[0].Name != "db.home.example.com" {
		t.Errorf("expected Name=%q, got %q", "db.home.example.com", hostnames[0].Name)
	}
}

func TestSupportedPlatforms(t *testing.T) {
	src := New()
	platforms := src.SupportedPlatforms()
	if len(platforms) != 1 {
		t.Fatalf("expected 1 supported platform, got %d", len(platforms))
	}
	if platforms[0] != workload.PlatformProxmox {
		t.Errorf("expected PlatformProxmox, got %q", platforms[0])
	}
}

func TestSupportsDiscovery(t *testing.T) {
	src := New()
	if src.SupportsDiscovery() {
		t.Error("expected SupportsDiscovery() to return false")
	}
}

func TestName(t *testing.T) {
	src := New()
	if src.Name() != "proxmox" {
		t.Errorf("expected Name()=%q, got %q", "proxmox", src.Name())
	}
}
