package kubernetes

import (
	"context"
	"log/slog"
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/source"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/workload"
)

func testLogger() *slog.Logger {
	return slog.Default()
}

func TestKubernetes_Name(t *testing.T) {
	k := New()
	if got := k.Name(); got != "kubernetes" {
		t.Errorf("Name() = %q, want %q", got, "kubernetes")
	}
}

func TestKubernetes_SupportedPlatforms(t *testing.T) {
	k := New()
	platforms := k.SupportedPlatforms()
	if len(platforms) != 1 {
		t.Fatalf("SupportedPlatforms() returned %d platforms, want 1", len(platforms))
	}
	if platforms[0] != workload.PlatformKubernetes {
		t.Errorf("SupportedPlatforms()[0] = %q, want %q", platforms[0], workload.PlatformKubernetes)
	}
}

func TestKubernetes_SupportsDiscovery(t *testing.T) {
	k := New()
	if k.SupportsDiscovery() {
		t.Error("SupportsDiscovery() = true, want false")
	}
}

func TestKubernetes_Discover(t *testing.T) {
	k := New()
	hostnames, err := k.Discover(context.Background())
	if err != nil {
		t.Errorf("Discover() error = %v", err)
	}
	if hostnames != nil {
		t.Errorf("Discover() = %v, want nil", hostnames)
	}
}

func TestKubernetes_Extract_Ingress(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:        "uid-123",
		Name:      "default/my-ingress",
		Namespace: "default",
		Labels: map[string]string{
			"app": "myapp",
		},
		Annotations: map[string]string{
			"kubernetes.io/ingress.class": "traefik",
		},
		Platform:  workload.PlatformKubernetes,
		Kind:      workload.KindIngress,
		Hostnames: []string{"app.example.com", "www.example.com"},
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 2 {
		t.Fatalf("Extract() returned %d hostnames, want 2", len(hostnames))
	}

	assertHostname(t, hostnames[0], "app.example.com", "kubernetes", "ingress:default/my-ingress")
	assertHostname(t, hostnames[1], "www.example.com", "kubernetes", "ingress:default/my-ingress")
}

func TestKubernetes_Extract_IngressRoute(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:        "uid-456",
		Name:      "default/my-ingressroute",
		Namespace: "default",
		Platform:  workload.PlatformKubernetes,
		Kind:      workload.KindIngressRoute,
		Hostnames: []string{"api.example.com"},
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 1 {
		t.Fatalf("Extract() returned %d hostnames, want 1", len(hostnames))
	}

	assertHostname(t, hostnames[0], "api.example.com", "kubernetes", "ingressroute:default/my-ingressroute")
}

func TestKubernetes_Extract_HTTPRoute(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:        "uid-789",
		Name:      "default/my-httproute",
		Namespace: "default",
		Platform:  workload.PlatformKubernetes,
		Kind:      workload.KindHTTPRoute,
		Hostnames: []string{"gw.example.com", "api.example.com"},
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 2 {
		t.Fatalf("Extract() returned %d hostnames, want 2", len(hostnames))
	}

	assertHostname(t, hostnames[0], "gw.example.com", "kubernetes", "httproute:default/my-httproute")
	assertHostname(t, hostnames[1], "api.example.com", "kubernetes", "httproute:default/my-httproute")
}

func TestKubernetes_Extract_Service(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:        "uid-svc",
		Name:      "kube-system/traefik",
		Namespace: "kube-system",
		Annotations: map[string]string{
			"external-dns.alpha.kubernetes.io/hostname": "lb.example.com",
		},
		Platform:  workload.PlatformKubernetes,
		Kind:      workload.KindK8sService,
		Hostnames: []string{"lb.example.com"},
		Metadata: map[string]string{
			"type":      "LoadBalancer",
			"clusterIP": "10.43.0.100",
		},
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 1 {
		t.Fatalf("Extract() returned %d hostnames, want 1", len(hostnames))
	}

	assertHostname(t, hostnames[0], "lb.example.com", "kubernetes", "k8s-service:kube-system/traefik")
}

func TestKubernetes_Extract_NoHostnames(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:       "uid-empty",
		Name:     "default/no-hosts",
		Platform: workload.PlatformKubernetes,
		Kind:     workload.KindIngress,
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 0 {
		t.Errorf("Extract() returned %d hostnames, want 0", len(hostnames))
	}
}

func TestKubernetes_Extract_EmptyHostnameSkipped(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:        "uid-partial",
		Name:      "default/partial",
		Platform:  workload.PlatformKubernetes,
		Kind:      workload.KindIngress,
		Hostnames: []string{"app.example.com", "", "www.example.com"},
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 2 {
		t.Fatalf("Extract() returned %d hostnames, want 2", len(hostnames))
	}

	assertHostname(t, hostnames[0], "app.example.com", "kubernetes", "ingress:default/partial")
	assertHostname(t, hostnames[1], "www.example.com", "kubernetes", "ingress:default/partial")
}

func TestKubernetes_Extract_DisabledAnnotation(t *testing.T) {
	k := New(WithLogger(testLogger()))

	tests := []struct {
		name  string
		value string
	}{
		{"lowercase false", "false"},
		{"uppercase FALSE", "FALSE"},
		{"mixed case False", "False"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := workload.Workload{
				ID:   "uid-disabled",
				Name: "default/disabled",
				Annotations: map[string]string{
					AnnotationEnabled: tt.value,
				},
				Platform:  workload.PlatformKubernetes,
				Kind:      workload.KindIngress,
				Hostnames: []string{"should-not-appear.example.com"},
			}

			hostnames, err := k.Extract(context.Background(), w)
			if err != nil {
				t.Fatalf("Extract() error = %v", err)
			}

			if len(hostnames) != 0 {
				t.Errorf("Extract() returned %d hostnames, want 0 (disabled)", len(hostnames))
			}
		})
	}
}

func TestKubernetes_Extract_EnabledAnnotation(t *testing.T) {
	k := New(WithLogger(testLogger()))

	tests := []struct {
		name        string
		annotations map[string]string
	}{
		{"explicitly true", map[string]string{AnnotationEnabled: "true"}},
		{"absent annotation", map[string]string{}},
		{"nil annotations", nil},
		{"non-false value", map[string]string{AnnotationEnabled: "yes"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := workload.Workload{
				ID:          "uid-enabled",
				Name:        "default/enabled",
				Annotations: tt.annotations,
				Platform:    workload.PlatformKubernetes,
				Kind:        workload.KindIngress,
				Hostnames:   []string{"app.example.com"},
			}

			hostnames, err := k.Extract(context.Background(), w)
			if err != nil {
				t.Fatalf("Extract() error = %v", err)
			}

			if len(hostnames) != 1 {
				t.Errorf("Extract() returned %d hostnames, want 1", len(hostnames))
			}
		})
	}
}

func TestKubernetes_Extract_RecordHints(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:   "uid-hints",
		Name: "default/with-hints",
		Annotations: map[string]string{
			AnnotationRecordType: "A",
			AnnotationTarget:     "10.30.0.100",
			AnnotationTTL:        "300",
			AnnotationProvider:   "internal-dns",
		},
		Platform:  workload.PlatformKubernetes,
		Kind:      workload.KindIngress,
		Hostnames: []string{"app.example.com", "www.example.com"},
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 2 {
		t.Fatalf("Extract() returned %d hostnames, want 2", len(hostnames))
	}

	// Both hostnames should have the same hints.
	for i, h := range hostnames {
		if h.RecordHints == nil {
			t.Fatalf("hostnames[%d].RecordHints is nil", i)
		}
		if h.RecordHints.Type != "A" {
			t.Errorf("hostnames[%d].RecordHints.Type = %q, want %q", i, h.RecordHints.Type, "A")
		}
		if h.RecordHints.Target != "10.30.0.100" {
			t.Errorf("hostnames[%d].RecordHints.Target = %q, want %q", i, h.RecordHints.Target, "10.30.0.100")
		}
		if h.RecordHints.TTL != 300 {
			t.Errorf("hostnames[%d].RecordHints.TTL = %d, want %d", i, h.RecordHints.TTL, 300)
		}
		if h.RecordHints.Provider != "internal-dns" {
			t.Errorf("hostnames[%d].RecordHints.Provider = %q, want %q", i, h.RecordHints.Provider, "internal-dns")
		}
	}

	// Verify hints are independent copies (not shared pointers).
	hostnames[0].RecordHints.Target = "modified"
	if hostnames[1].RecordHints.Target == "modified" {
		t.Error("RecordHints are shared between hostnames — should be independent copies")
	}
}

func TestKubernetes_Extract_PartialRecordHints(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:   "uid-partial-hints",
		Name: "default/partial-hints",
		Annotations: map[string]string{
			AnnotationProvider: "cloudflare",
		},
		Platform:  workload.PlatformKubernetes,
		Kind:      workload.KindIngress,
		Hostnames: []string{"app.example.com"},
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 1 {
		t.Fatalf("Extract() returned %d hostnames, want 1", len(hostnames))
	}

	h := hostnames[0]
	if h.RecordHints == nil {
		t.Fatal("RecordHints is nil, want non-nil with Provider set")
	}
	if h.RecordHints.Provider != "cloudflare" {
		t.Errorf("RecordHints.Provider = %q, want %q", h.RecordHints.Provider, "cloudflare")
	}
	if h.RecordHints.Type != "" {
		t.Errorf("RecordHints.Type = %q, want empty", h.RecordHints.Type)
	}
}

func TestKubernetes_Extract_ProxiedAnnotation(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:   "uid-proxied",
		Name: "default/proxied",
		Annotations: map[string]string{
			AnnotationProvider: "cloudflare",
			AnnotationProxied:  "true",
		},
		Platform:  workload.PlatformKubernetes,
		Kind:      workload.KindIngress,
		Hostnames: []string{"app.example.com"},
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 1 {
		t.Fatalf("Extract() returned %d hostnames, want 1", len(hostnames))
	}

	h := hostnames[0]
	if h.RecordHints == nil {
		t.Fatal("RecordHints is nil")
	}
	if h.RecordHints.Metadata == nil {
		t.Fatal("RecordHints.Metadata is nil")
	}
	if h.RecordHints.Metadata["proxied"] != "true" {
		t.Errorf("RecordHints.Metadata[proxied] = %q, want %q", h.RecordHints.Metadata["proxied"], "true")
	}
}

func TestKubernetes_Extract_InvalidTTLIgnored(t *testing.T) {
	k := New(WithLogger(testLogger()))

	tests := []struct {
		name string
		ttl  string
	}{
		{"not a number", "abc"},
		{"zero", "0"},
		{"negative", "-1"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := workload.Workload{
				ID:   "uid-bad-ttl",
				Name: "default/bad-ttl",
				Annotations: map[string]string{
					AnnotationTTL:    tt.ttl,
					AnnotationTarget: "10.0.0.1", // Need at least one hint to get non-nil RecordHints
				},
				Platform:  workload.PlatformKubernetes,
				Kind:      workload.KindIngress,
				Hostnames: []string{"app.example.com"},
			}

			hostnames, err := k.Extract(context.Background(), w)
			if err != nil {
				t.Fatalf("Extract() error = %v", err)
			}

			if len(hostnames) != 1 {
				t.Fatalf("Extract() returned %d hostnames, want 1", len(hostnames))
			}

			// TTL should be 0 (default/unset) since the annotation was invalid.
			if hostnames[0].RecordHints != nil && hostnames[0].RecordHints.TTL != 0 {
				t.Errorf("RecordHints.TTL = %d, want 0 (invalid TTL should be ignored)", hostnames[0].RecordHints.TTL)
			}
		})
	}
}

func TestKubernetes_Extract_NoHintAnnotations(t *testing.T) {
	k := New(WithLogger(testLogger()))

	w := workload.Workload{
		ID:   "uid-no-hints",
		Name: "default/no-hints",
		Annotations: map[string]string{
			"kubernetes.io/ingress.class": "traefik",
			"something-else":              "value",
		},
		Platform:  workload.PlatformKubernetes,
		Kind:      workload.KindIngress,
		Hostnames: []string{"app.example.com"},
	}

	hostnames, err := k.Extract(context.Background(), w)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if len(hostnames) != 1 {
		t.Fatalf("Extract() returned %d hostnames, want 1", len(hostnames))
	}

	// No dnsweaver.dev/* annotations → nil RecordHints.
	if hostnames[0].RecordHints != nil {
		t.Errorf("RecordHints = %+v, want nil (no hint annotations)", hostnames[0].RecordHints)
	}
}

func TestKubernetes_Extract_RecordTypeCaseNormalization(t *testing.T) {
	k := New(WithLogger(testLogger()))

	tests := []struct {
		input string
		want  string
	}{
		{"a", "A"},
		{"aaaa", "AAAA"},
		{"cname", "CNAME"},
		{"CNAME", "CNAME"},
		{"Srv", "SRV"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			w := workload.Workload{
				ID:   "uid-type",
				Name: "default/type-test",
				Annotations: map[string]string{
					AnnotationRecordType: tt.input,
				},
				Platform:  workload.PlatformKubernetes,
				Kind:      workload.KindIngress,
				Hostnames: []string{"app.example.com"},
			}

			hostnames, err := k.Extract(context.Background(), w)
			if err != nil {
				t.Fatalf("Extract() error = %v", err)
			}

			if hostnames[0].RecordHints == nil {
				t.Fatal("RecordHints is nil")
			}
			if hostnames[0].RecordHints.Type != tt.want {
				t.Errorf("RecordHints.Type = %q, want %q", hostnames[0].RecordHints.Type, tt.want)
			}
		})
	}
}

// assertHostname is a test helper that checks hostname fields.
func assertHostname(t *testing.T, h source.Hostname, wantName, wantSource, wantRouter string) {
	t.Helper()
	if h.Name != wantName {
		t.Errorf("Hostname.Name = %q, want %q", h.Name, wantName)
	}
	if h.Source != wantSource {
		t.Errorf("Hostname.Source = %q, want %q", h.Source, wantSource)
	}
	if h.Router != wantRouter {
		t.Errorf("Hostname.Router = %q, want %q", h.Router, wantRouter)
	}
}
