package incus

import "testing"

func TestResolveIP(t *testing.T) {
	tests := []struct {
		name string
		inst Instance
		want string
	}{
		{
			name: "nil state",
			inst: Instance{Name: "x"},
			want: "",
		},
		{
			name: "global ipv4 on eth0",
			inst: Instance{State: &InstanceState{Network: map[string]InstanceNetwork{
				"eth0": {Addresses: []InstanceAddress{
					{Family: "inet", Address: "192.168.1.10", Scope: "global"},
				}},
			}}},
			want: "192.168.1.10",
		},
		{
			name: "skips loopback interface",
			inst: Instance{State: &InstanceState{Network: map[string]InstanceNetwork{
				"lo": {Addresses: []InstanceAddress{
					{Family: "inet", Address: "127.0.0.1", Scope: "local"},
				}},
				"eth0": {Addresses: []InstanceAddress{
					{Family: "inet", Address: "10.0.0.7", Scope: "global"},
				}},
			}}},
			want: "10.0.0.7",
		},
		{
			name: "skips link-local and ipv6, picks global ipv4",
			inst: Instance{State: &InstanceState{Network: map[string]InstanceNetwork{
				"eth0": {Addresses: []InstanceAddress{
					{Family: "inet6", Address: "fe80::1", Scope: "link"},
					{Family: "inet", Address: "169.254.1.1", Scope: "link"},
					{Family: "inet", Address: "10.1.2.3", Scope: "global"},
				}},
			}}},
			want: "10.1.2.3",
		},
		{
			name: "deterministic across multiple interfaces (sorted)",
			inst: Instance{State: &InstanceState{Network: map[string]InstanceNetwork{
				"eth1": {Addresses: []InstanceAddress{
					{Family: "inet", Address: "10.0.0.20", Scope: "global"},
				}},
				"eth0": {Addresses: []InstanceAddress{
					{Family: "inet", Address: "10.0.0.10", Scope: "global"},
				}},
			}}},
			want: "10.0.0.10",
		},
		{
			name: "no routable address",
			inst: Instance{State: &InstanceState{Network: map[string]InstanceNetwork{
				"eth0": {Addresses: []InstanceAddress{
					{Family: "inet", Address: "127.0.0.1", Scope: "global"},
				}},
			}}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveIP(tt.inst); got != tt.want {
				t.Errorf("ResolveIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsNonRoutableIP(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", false},
		{"192.168.1.1", false},
		{"172.16.5.5", false},
		{"100.64.0.1", false}, // CGNAT / Tailscale kept
		{"127.0.0.1", true},
		{"169.254.1.1", true},
		{"0.0.0.0", true},
		{"224.0.0.1", true},
		{"203.0.113.5", true}, // TEST-NET-3
		{"", true},
		{"not-an-ip", true},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			if got := isNonRoutableIP(tt.ip); got != tt.want {
				t.Errorf("isNonRoutableIP(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}
