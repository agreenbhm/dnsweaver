package provider

import "testing"

func TestRecordEquals(t *testing.T) {
	tests := []struct {
		name     string
		a        Record
		b        Record
		expected bool
	}{
		{
			name: "identical A records",
			a: Record{
				Hostname: "app.example.com",
				Type:     RecordTypeA,
				Target:   "10.0.0.1",
				TTL:      300,
			},
			b: Record{
				Hostname: "app.example.com",
				Type:     RecordTypeA,
				Target:   "10.0.0.1",
				TTL:      300,
			},
			expected: true,
		},
		{
			name: "different hostnames",
			a: Record{
				Hostname: "app1.example.com",
				Type:     RecordTypeA,
				Target:   "10.0.0.1",
				TTL:      300,
			},
			b: Record{
				Hostname: "app2.example.com",
				Type:     RecordTypeA,
				Target:   "10.0.0.1",
				TTL:      300,
			},
			expected: false,
		},
		{
			name: "different types",
			a: Record{
				Hostname: "app.example.com",
				Type:     RecordTypeA,
				Target:   "10.0.0.1",
				TTL:      300,
			},
			b: Record{
				Hostname: "app.example.com",
				Type:     RecordTypeAAAA,
				Target:   "::1",
				TTL:      300,
			},
			expected: false,
		},
		{
			name: "different TTL",
			a: Record{
				Hostname: "app.example.com",
				Type:     RecordTypeA,
				Target:   "10.0.0.1",
				TTL:      300,
			},
			b: Record{
				Hostname: "app.example.com",
				Type:     RecordTypeA,
				Target:   "10.0.0.1",
				TTL:      600,
			},
			expected: false,
		},
		{
			name: "identical SRV records",
			a: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV: &SRVData{
					Priority: 10,
					Weight:   5,
					Port:     25565,
				},
			},
			b: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV: &SRVData{
					Priority: 10,
					Weight:   5,
					Port:     25565,
				},
			},
			expected: true,
		},
		{
			name: "SRV records with different priority",
			a: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV: &SRVData{
					Priority: 10,
					Weight:   5,
					Port:     25565,
				},
			},
			b: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV: &SRVData{
					Priority: 20,
					Weight:   5,
					Port:     25565,
				},
			},
			expected: false,
		},
		{
			name: "SRV records with different weight",
			a: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV: &SRVData{
					Priority: 10,
					Weight:   5,
					Port:     25565,
				},
			},
			b: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV: &SRVData{
					Priority: 10,
					Weight:   10,
					Port:     25565,
				},
			},
			expected: false,
		},
		{
			name: "SRV records with different port",
			a: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV: &SRVData{
					Priority: 10,
					Weight:   5,
					Port:     25565,
				},
			},
			b: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV: &SRVData{
					Priority: 10,
					Weight:   5,
					Port:     25566,
				},
			},
			expected: false,
		},
		{
			name: "SRV record with nil vs non-nil SRV data",
			a: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV:      nil,
			},
			b: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV: &SRVData{
					Priority: 10,
					Weight:   5,
					Port:     25565,
				},
			},
			expected: false,
		},
		{
			name: "SRV records with both nil SRV data",
			a: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV:      nil,
			},
			b: Record{
				Hostname: "_minecraft._tcp.example.com",
				Type:     RecordTypeSRV,
				Target:   "mc.example.com",
				TTL:      3600,
				SRV:      nil,
			},
			expected: true,
		},
		{
			name: "provider ID should not affect equality",
			a: Record{
				Hostname:   "app.example.com",
				Type:       RecordTypeA,
				Target:     "10.0.0.1",
				TTL:        300,
				ProviderID: "record-123",
			},
			b: Record{
				Hostname:   "app.example.com",
				Type:       RecordTypeA,
				Target:     "10.0.0.1",
				TTL:        300,
				ProviderID: "record-456",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RecordEquals(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("RecordEquals() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestRecordTypeConstants(t *testing.T) {
	// Verify record type constants are correct
	if RecordTypeA != "A" {
		t.Errorf("RecordTypeA = %q, expected %q", RecordTypeA, "A")
	}
	if RecordTypeAAAA != "AAAA" {
		t.Errorf("RecordTypeAAAA = %q, expected %q", RecordTypeAAAA, "AAAA")
	}
	if RecordTypeCNAME != "CNAME" {
		t.Errorf("RecordTypeCNAME = %q, expected %q", RecordTypeCNAME, "CNAME")
	}
	if RecordTypeTXT != "TXT" {
		t.Errorf("RecordTypeTXT = %q, expected %q", RecordTypeTXT, "TXT")
	}
	if RecordTypeSRV != "SRV" {
		t.Errorf("RecordTypeSRV = %q, expected %q", RecordTypeSRV, "SRV")
	}
}

func TestCapabilities_SupportsRecordType(t *testing.T) {
	tests := []struct {
		name     string
		caps     Capabilities
		rt       RecordType
		expected bool
	}{
		{
			name: "full capabilities - A record",
			caps: Capabilities{
				SupportedRecordTypes: []RecordType{RecordTypeA, RecordTypeAAAA, RecordTypeCNAME, RecordTypeSRV, RecordTypeTXT},
			},
			rt:       RecordTypeA,
			expected: true,
		},
		{
			name: "full capabilities - SRV record",
			caps: Capabilities{
				SupportedRecordTypes: []RecordType{RecordTypeA, RecordTypeAAAA, RecordTypeCNAME, RecordTypeSRV, RecordTypeTXT},
			},
			rt:       RecordTypeSRV,
			expected: true,
		},
		{
			name: "limited capabilities - A only",
			caps: Capabilities{
				SupportedRecordTypes: []RecordType{RecordTypeA},
			},
			rt:       RecordTypeA,
			expected: true,
		},
		{
			name: "limited capabilities - missing AAAA",
			caps: Capabilities{
				SupportedRecordTypes: []RecordType{RecordTypeA, RecordTypeCNAME},
			},
			rt:       RecordTypeAAAA,
			expected: false,
		},
		{
			name: "empty capabilities",
			caps: Capabilities{
				SupportedRecordTypes: []RecordType{},
			},
			rt:       RecordTypeA,
			expected: false,
		},
		{
			name: "nil capabilities",
			caps: Capabilities{
				SupportedRecordTypes: nil,
			},
			rt:       RecordTypeA,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.caps.SupportsRecordType(tt.rt)
			if result != tt.expected {
				t.Errorf("SupportsRecordType(%s) = %v, expected %v", tt.rt, result, tt.expected)
			}
		})
	}
}

func TestCapabilities_Defaults(t *testing.T) {
	// Test that zero-value Capabilities are restrictive (safe defaults)
	var caps Capabilities

	if caps.SupportsOwnershipTXT {
		t.Error("zero-value SupportsOwnershipTXT should be false")
	}
	if caps.SupportsNativeUpdate {
		t.Error("zero-value SupportsNativeUpdate should be false")
	}
	if len(caps.SupportedRecordTypes) != 0 {
		t.Error("zero-value SupportedRecordTypes should be empty")
	}
	if caps.SupportsRecordType(RecordTypeA) {
		t.Error("zero-value caps should not support any record type")
	}
}

func TestMakeOwnershipValue(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		expected   string
	}{
		{
			name:       "empty instance ID returns legacy format",
			instanceID: "",
			expected:   "heritage=dnsweaver",
		},
		{
			name:       "instance ID is included",
			instanceID: "pi5-dns",
			expected:   "heritage=dnsweaver,instance=pi5-dns",
		},
		{
			name:       "instance ID with dots and underscores",
			instanceID: "k8s_node.01",
			expected:   "heritage=dnsweaver,instance=k8s_node.01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MakeOwnershipValue(tt.instanceID)
			if result != tt.expected {
				t.Errorf("MakeOwnershipValue(%q) = %q, expected %q", tt.instanceID, result, tt.expected)
			}
		})
	}
}

func TestParseOwnershipValue(t *testing.T) {
	tests := []struct {
		name           string
		value          string
		wantOwned      bool
		wantInstanceID string
	}{
		{
			name:           "legacy format",
			value:          "heritage=dnsweaver",
			wantOwned:      true,
			wantInstanceID: "",
		},
		{
			name:           "with instance ID",
			value:          "heritage=dnsweaver,instance=pi5-dns",
			wantOwned:      true,
			wantInstanceID: "pi5-dns",
		},
		{
			name:           "with complex instance ID",
			value:          "heritage=dnsweaver,instance=k8s-node_01.prod",
			wantOwned:      true,
			wantInstanceID: "k8s-node_01.prod",
		},
		{
			name:           "not a dnsweaver record",
			value:          "some-other-value",
			wantOwned:      false,
			wantInstanceID: "",
		},
		{
			name:           "partial heritage match",
			value:          "heritage=dnsweaver-fork",
			wantOwned:      false,
			wantInstanceID: "",
		},
		{
			name:           "empty value",
			value:          "",
			wantOwned:      false,
			wantInstanceID: "",
		},
		{
			name:           "heritage prefix only with comma but no instance",
			value:          "heritage=dnsweaver,other=field",
			wantOwned:      true,
			wantInstanceID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOwned, gotInstanceID := ParseOwnershipValue(tt.value)
			if gotOwned != tt.wantOwned {
				t.Errorf("ParseOwnershipValue(%q) isOwned = %v, want %v", tt.value, gotOwned, tt.wantOwned)
			}
			if gotInstanceID != tt.wantInstanceID {
				t.Errorf("ParseOwnershipValue(%q) instanceID = %q, want %q", tt.value, gotInstanceID, tt.wantInstanceID)
			}
		})
	}
}

func TestMatchesOwnership(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		ourInstanceID string
		expected      bool
	}{
		{
			name:          "legacy record matches empty instance ID",
			value:         "heritage=dnsweaver",
			ourInstanceID: "",
			expected:      true,
		},
		{
			name:          "legacy record does NOT match non-empty instance ID",
			value:         "heritage=dnsweaver",
			ourInstanceID: "pi5-dns",
			expected:      false,
		},
		{
			name:          "instance record matches same instance ID",
			value:         "heritage=dnsweaver,instance=pi5-dns",
			ourInstanceID: "pi5-dns",
			expected:      true,
		},
		{
			name:          "instance record does NOT match different instance ID",
			value:         "heritage=dnsweaver,instance=pi5-dns",
			ourInstanceID: "k8s-node",
			expected:      false,
		},
		{
			name:          "instance record does NOT match empty instance ID",
			value:         "heritage=dnsweaver,instance=pi5-dns",
			ourInstanceID: "",
			expected:      false,
		},
		{
			name:          "not a dnsweaver record",
			value:         "some-other-value",
			ourInstanceID: "",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchesOwnership(tt.value, tt.ourInstanceID)
			if result != tt.expected {
				t.Errorf("MatchesOwnership(%q, %q) = %v, expected %v", tt.value, tt.ourInstanceID, result, tt.expected)
			}
		})
	}
}

func TestIsDnsweaverOwned(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{
			name:     "legacy format is owned",
			value:    "heritage=dnsweaver",
			expected: true,
		},
		{
			name:     "instance format is owned",
			value:    "heritage=dnsweaver,instance=pi5-dns",
			expected: true,
		},
		{
			name:     "other value is not owned",
			value:    "something-else",
			expected: false,
		},
		{
			name:     "empty is not owned",
			value:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDnsweaverOwned(tt.value)
			if result != tt.expected {
				t.Errorf("IsDnsweaverOwned(%q) = %v, expected %v", tt.value, result, tt.expected)
			}
		})
	}
}

func TestOwnershipRecord_WithInstanceID(t *testing.T) {
	// Legacy (no instance ID)
	r := OwnershipRecord("app.example.com", 300, "")
	if r.Hostname != "_dnsweaver.app.example.com" {
		t.Errorf("expected hostname '_dnsweaver.app.example.com', got %q", r.Hostname)
	}
	if r.Target != "heritage=dnsweaver" {
		t.Errorf("expected target 'heritage=dnsweaver', got %q", r.Target)
	}

	// With instance ID
	r2 := OwnershipRecord("app.example.com", 300, "pi5-dns")
	if r2.Hostname != "_dnsweaver.app.example.com" {
		t.Errorf("expected hostname '_dnsweaver.app.example.com', got %q", r2.Hostname)
	}
	if r2.Target != "heritage=dnsweaver,instance=pi5-dns" {
		t.Errorf("expected target 'heritage=dnsweaver,instance=pi5-dns', got %q", r2.Target)
	}
}
