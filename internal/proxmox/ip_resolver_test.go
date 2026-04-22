package proxmox

import "testing"

func TestParseLXCNet0IP(t *testing.T) {
	tests := []struct {
		name   string
		net0   string
		wantIP string
	}{
		{
			name:   "static IP with CIDR",
			net0:   "name=eth0,bridge=vmbr0,hwaddr=AA:BB:CC:DD:EE:FF,ip=192.0.2.50/24,ip6=auto",
			wantIP: "192.0.2.50",
		},
		{
			name:   "DHCP returns empty",
			net0:   "name=eth0,bridge=vmbr0,ip=dhcp",
			wantIP: "",
		},
		{
			name:   "no ip field returns empty",
			net0:   "name=eth0,bridge=vmbr0",
			wantIP: "",
		},
		{
			name:   "IP without CIDR",
			net0:   "name=eth0,bridge=vmbr0,ip=192.168.1.100",
			wantIP: "192.168.1.100",
		},
		{
			name:   "empty net0",
			net0:   "",
			wantIP: "",
		},
		{
			name:   "multiple network params, ip first",
			net0:   "ip=172.16.0.5/16,name=eth0,bridge=vmbr1",
			wantIP: "172.16.0.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLXCNet0IP(tt.net0)
			if got != tt.wantIP {
				t.Errorf("parseLXCNet0IP(%q) = %q, want %q", tt.net0, got, tt.wantIP)
			}
		})
	}
}

func TestIsLoopback(t *testing.T) {
	if !isLoopback("lo") {
		t.Error("expected lo to be loopback")
	}
	if !isLoopback("lo0") {
		t.Error("expected lo0 to be loopback")
	}
	if isLoopback("eth0") {
		t.Error("expected eth0 to not be loopback")
	}
}

func TestIsLoopbackIP(t *testing.T) {
	if !isLoopbackIP("127.0.0.1") {
		t.Error("expected 127.0.0.1 to be loopback")
	}
	if !isLoopbackIP("127.1.2.3") {
		t.Error("expected 127.1.2.3 to be loopback")
	}
	if isLoopbackIP("192.0.2.50") {
		t.Error("expected 192.0.2.50 to not be loopback")
	}
}
