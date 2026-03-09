package kubernetes

import (
	"testing"
)

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{"nil annotations", nil, true},
		{"empty annotations", map[string]string{}, true},
		{"absent key", map[string]string{"other": "value"}, true},
		{"true", map[string]string{AnnotationEnabled: "true"}, true},
		{"yes", map[string]string{AnnotationEnabled: "yes"}, true},
		{"false", map[string]string{AnnotationEnabled: "false"}, false},
		{"FALSE", map[string]string{AnnotationEnabled: "FALSE"}, false},
		{"False", map[string]string{AnnotationEnabled: "False"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEnabled(tt.annotations); got != tt.want {
				t.Errorf("isEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseRecordHints(t *testing.T) {
	t.Run("nil annotations", func(t *testing.T) {
		hints := parseRecordHints(nil)
		if hints != nil {
			t.Errorf("parseRecordHints(nil) = %+v, want nil", hints)
		}
	})

	t.Run("empty annotations", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{})
		if hints != nil {
			t.Errorf("parseRecordHints({}) = %+v, want nil", hints)
		}
	})

	t.Run("no dnsweaver annotations", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{
			"kubernetes.io/ingress.class": "traefik",
		})
		if hints != nil {
			t.Errorf("parseRecordHints() = %+v, want nil", hints)
		}
	})

	t.Run("full hints", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{
			AnnotationRecordType: "A",
			AnnotationTarget:     "10.30.0.100",
			AnnotationTTL:        "300",
			AnnotationProvider:   "internal-dns",
		})

		if hints == nil {
			t.Fatal("parseRecordHints() = nil, want non-nil")
		}
		if hints.Type != "A" {
			t.Errorf("Type = %q, want %q", hints.Type, "A")
		}
		if hints.Target != "10.30.0.100" {
			t.Errorf("Target = %q, want %q", hints.Target, "10.30.0.100")
		}
		if hints.TTL != 300 {
			t.Errorf("TTL = %d, want %d", hints.TTL, 300)
		}
		if hints.Provider != "internal-dns" {
			t.Errorf("Provider = %q, want %q", hints.Provider, "internal-dns")
		}
	})

	t.Run("partial hints - provider only", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{
			AnnotationProvider: "cloudflare",
		})

		if hints == nil {
			t.Fatal("parseRecordHints() = nil, want non-nil")
		}
		if hints.Provider != "cloudflare" {
			t.Errorf("Provider = %q, want %q", hints.Provider, "cloudflare")
		}
		if hints.Type != "" {
			t.Errorf("Type = %q, want empty", hints.Type)
		}
		if hints.TTL != 0 {
			t.Errorf("TTL = %d, want 0", hints.TTL)
		}
	})

	t.Run("record type normalization", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{
			AnnotationRecordType: "cname",
		})

		if hints == nil {
			t.Fatal("parseRecordHints() = nil, want non-nil")
		}
		if hints.Type != "CNAME" {
			t.Errorf("Type = %q, want %q", hints.Type, "CNAME")
		}
	})

	t.Run("invalid TTL ignored", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{
			AnnotationTTL:    "not-a-number",
			AnnotationTarget: "10.0.0.1", // Include another hint to get non-nil result
		})

		if hints == nil {
			t.Fatal("parseRecordHints() = nil, want non-nil")
		}
		if hints.TTL != 0 {
			t.Errorf("TTL = %d, want 0 (invalid value should be ignored)", hints.TTL)
		}
		if hints.Target != "10.0.0.1" {
			t.Errorf("Target = %q, want %q", hints.Target, "10.0.0.1")
		}
	})

	t.Run("zero TTL ignored", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{
			AnnotationTTL:    "0",
			AnnotationTarget: "10.0.0.1",
		})

		if hints == nil {
			t.Fatal("parseRecordHints() = nil, want non-nil")
		}
		if hints.TTL != 0 {
			t.Errorf("TTL = %d, want 0", hints.TTL)
		}
	})

	t.Run("negative TTL ignored", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{
			AnnotationTTL:    "-10",
			AnnotationTarget: "10.0.0.1",
		})

		if hints == nil {
			t.Fatal("parseRecordHints() = nil, want non-nil")
		}
		if hints.TTL != 0 {
			t.Errorf("TTL = %d, want 0", hints.TTL)
		}
	})

	t.Run("proxied metadata", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{
			AnnotationProxied: "true",
		})

		if hints == nil {
			t.Fatal("parseRecordHints() = nil, want non-nil")
		}
		if hints.Metadata == nil {
			t.Fatal("Metadata is nil")
		}
		if hints.Metadata["proxied"] != "true" {
			t.Errorf("Metadata[proxied] = %q, want %q", hints.Metadata["proxied"], "true")
		}
	})

	t.Run("empty values ignored", func(t *testing.T) {
		hints := parseRecordHints(map[string]string{
			AnnotationRecordType: "",
			AnnotationTarget:     "",
			AnnotationTTL:        "",
			AnnotationProvider:   "",
			AnnotationProxied:    "",
		})

		if hints != nil {
			t.Errorf("parseRecordHints() = %+v, want nil (all empty values)", hints)
		}
	})
}
