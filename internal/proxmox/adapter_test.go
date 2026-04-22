package proxmox

import (
	"testing"
)

func TestToWorkload_VM(t *testing.T) {
	r := ClusterResource{
		VMID:   100,
		Name:   "web-server",
		Node:   "pve-00",
		Type:   "qemu",
		Status: "running",
		Tags:   "dnsweaver;web",
	}

	w := toWorkload(r, "10.1.20.100")

	if w.ID != "pve-00/100" {
		t.Errorf("ID = %q, want %q", w.ID, "pve-00/100")
	}
	if w.Name != "web-server" {
		t.Errorf("Name = %q, want %q", w.Name, "web-server")
	}
	if string(w.Kind) != "vm" {
		t.Errorf("Kind = %q, want %q", w.Kind, "vm")
	}
	if string(w.Platform) != "proxmox" {
		t.Errorf("Platform = %q, want %q", w.Platform, "proxmox")
	}
	if w.Metadata["ip"] != "10.1.20.100" {
		t.Errorf("Metadata[ip] = %q, want %q", w.Metadata["ip"], "10.1.20.100")
	}
	if w.Metadata["node"] != "pve-00" {
		t.Errorf("Metadata[node] = %q, want %q", w.Metadata["node"], "pve-00")
	}
	if w.Labels["proxmox.tag/dnsweaver"] != "true" {
		t.Errorf("expected proxmox.tag/dnsweaver label, got %v", w.Labels)
	}
}

func TestToWorkload_LXC(t *testing.T) {
	r := ClusterResource{
		VMID:   200,
		Name:   "db-lxc",
		Node:   "pve-01",
		Type:   "lxc",
		Status: "running",
		Tags:   "",
	}

	w := toWorkload(r, "10.1.20.200")

	if string(w.Kind) != "lxc" {
		t.Errorf("Kind = %q, want %q", w.Kind, "lxc")
	}
}

func TestToWorkload_NoIP(t *testing.T) {
	r := ClusterResource{VMID: 100, Name: "vm", Node: "pve-00", Type: "qemu", Status: "running"}
	w := toWorkload(r, "")

	if _, ok := w.Metadata["ip"]; ok {
		t.Error("expected no 'ip' key in Metadata when IP is empty")
	}
}

func TestHasTagWithPrefix(t *testing.T) {
	tests := []struct {
		tags   string
		prefix string
		want   bool
	}{
		{"dnsweaver;web", "dnsweaver", true},
		{"dns;web", "dnsweaver", false},
		{"", "dnsweaver", false},
		{"web;dnsweaver-host=foo.example.com", "dnsweaver", true},
		{"other", "dnsweaver", false},
	}

	for _, tt := range tests {
		got := hasTagWithPrefix(tt.tags, tt.prefix)
		if got != tt.want {
			t.Errorf("hasTagWithPrefix(%q, %q) = %v, want %v", tt.tags, tt.prefix, got, tt.want)
		}
	}
}

func TestParseTags(t *testing.T) {
	labels := parseTags("dns;web;production")

	if labels["proxmox.tag/dns"] != "true" {
		t.Errorf("expected proxmox.tag/dns=true, got %v", labels)
	}
	if labels["proxmox.tag/web"] != "true" {
		t.Errorf("expected proxmox.tag/web=true, got %v", labels)
	}
	if labels["proxmox.tag/production"] != "true" {
		t.Errorf("expected proxmox.tag/production=true, got %v", labels)
	}

	empty := parseTags("")
	if len(empty) != 0 {
		t.Errorf("expected empty labels for empty tags, got %v", empty)
	}
}

func TestMatchesFilters(t *testing.T) {
	adapter := &WorkloadListerAdapter{
		cfg: AdapterConfig{
			StateFilter: "running",
			NodeFilter:  "pve-00",
			TagFilter:   "dnsweaver",
		},
	}

	tests := []struct {
		name string
		r    ClusterResource
		want bool
	}{
		{
			name: "all filters match",
			r:    ClusterResource{Status: "running", Node: "pve-00", Tags: "dnsweaver;web"},
			want: true,
		},
		{
			name: "wrong state",
			r:    ClusterResource{Status: "stopped", Node: "pve-00", Tags: "dnsweaver"},
			want: false,
		},
		{
			name: "wrong node",
			r:    ClusterResource{Status: "running", Node: "pve-01", Tags: "dnsweaver"},
			want: false,
		},
		{
			name: "missing tag",
			r:    ClusterResource{Status: "running", Node: "pve-00", Tags: "web"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.matchesFilters(tt.r)
			if got != tt.want {
				t.Errorf("matchesFilters = %v, want %v", got, tt.want)
			}
		})
	}
}
