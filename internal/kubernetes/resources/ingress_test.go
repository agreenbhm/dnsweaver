package resources

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/workload"
)

func TestConvertIngress(t *testing.T) {
	tests := []struct {
		name           string
		ingress        *networkingv1.Ingress
		wantHostnames  []string
		wantName       string
		wantNamespace  string
		wantKind       workload.Kind
		wantPlatform   workload.Platform
		wantLabelCount int
		wantAnnotCount int
	}{
		{
			name: "single host",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					UID:             types.UID("uid-1"),
					Name:            "my-app",
					Namespace:       "default",
					ResourceVersion: "123",
					Labels:          map[string]string{"app": "my-app"},
					Annotations:     map[string]string{"nginx.ingress.kubernetes.io/rewrite-target": "/"},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{Host: "myapp.example.com"},
					},
				},
			},
			wantHostnames:  []string{"myapp.example.com"},
			wantName:       "default/my-app",
			wantNamespace:  "default",
			wantKind:       workload.KindIngress,
			wantPlatform:   workload.PlatformKubernetes,
			wantLabelCount: 1,
			wantAnnotCount: 1,
		},
		{
			name: "multiple hosts",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("uid-2"),
					Name:      "multi-host",
					Namespace: "web",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{Host: "app.example.com"},
						{Host: "api.example.com"},
						{Host: "admin.example.com"},
					},
				},
			},
			wantHostnames: []string{"app.example.com", "api.example.com", "admin.example.com"},
			wantName:      "web/multi-host",
			wantNamespace: "web",
		},
		{
			name: "empty host rule skipped",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("uid-3"),
					Name:      "catch-all",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{Host: ""}, // catch-all, no specific hostname
						{Host: "real.example.com"},
					},
				},
			},
			wantHostnames: []string{"real.example.com"},
		},
		{
			name: "hosts from TLS section",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("uid-4"),
					Name:      "tls-ingress",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{
					TLS: []networkingv1.IngressTLS{
						{Hosts: []string{"secure.example.com", "also-secure.example.com"}},
					},
					Rules: []networkingv1.IngressRule{
						{Host: "secure.example.com"},
					},
				},
			},
			// secure.example.com from rules, also-secure.example.com from TLS (deduplicated)
			wantHostnames: []string{"secure.example.com", "also-secure.example.com"},
		},
		{
			name: "no rules",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("uid-5"),
					Name:      "empty",
					Namespace: "default",
				},
				Spec: networkingv1.IngressSpec{},
			},
			wantHostnames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := ConvertIngress(tt.ingress)

			// Check hostnames
			if len(w.Hostnames) != len(tt.wantHostnames) {
				t.Errorf("hostnames count = %d, want %d", len(w.Hostnames), len(tt.wantHostnames))
			}
			for i, want := range tt.wantHostnames {
				if i < len(w.Hostnames) && w.Hostnames[i] != want {
					t.Errorf("hostname[%d] = %q, want %q", i, w.Hostnames[i], want)
				}
			}

			// Check metadata
			if tt.wantName != "" && w.Name != tt.wantName {
				t.Errorf("name = %q, want %q", w.Name, tt.wantName)
			}
			if tt.wantNamespace != "" && w.Namespace != tt.wantNamespace {
				t.Errorf("namespace = %q, want %q", w.Namespace, tt.wantNamespace)
			}

			// Always Kubernetes platform and Ingress kind
			if w.Platform != workload.PlatformKubernetes {
				t.Errorf("platform = %q, want %q", w.Platform, workload.PlatformKubernetes)
			}
			if w.Kind != workload.KindIngress {
				t.Errorf("kind = %q, want %q", w.Kind, workload.KindIngress)
			}

			// Check ID
			if w.ID != string(tt.ingress.UID) {
				t.Errorf("ID = %q, want %q", w.ID, string(tt.ingress.UID))
			}

			// Check label/annotation copy isolation
			if tt.wantLabelCount > 0 && len(w.Labels) != tt.wantLabelCount {
				t.Errorf("labels count = %d, want %d", len(w.Labels), tt.wantLabelCount)
			}
			if tt.wantAnnotCount > 0 && len(w.Annotations) != tt.wantAnnotCount {
				t.Errorf("annotations count = %d, want %d", len(w.Annotations), tt.wantAnnotCount)
			}

			// Verify resourceVersion in metadata
			if w.Metadata["resourceVersion"] != tt.ingress.ResourceVersion {
				t.Errorf("metadata.resourceVersion = %q, want %q", w.Metadata["resourceVersion"], tt.ingress.ResourceVersion)
			}
		})
	}
}

func TestConvertIngress_MapIsolation(t *testing.T) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			UID:         types.UID("uid-iso"),
			Name:        "isolation-test",
			Namespace:   "default",
			Labels:      map[string]string{"original": "value"},
			Annotations: map[string]string{"key": "val"},
		},
	}

	w := ConvertIngress(ing)

	// Mutating the workload's maps should not affect the original ingress
	w.Labels["injected"] = "evil"
	w.Annotations["injected"] = "evil"

	if _, exists := ing.Labels["injected"]; exists {
		t.Error("mutating workload labels affected original ingress labels")
	}
	if _, exists := ing.Annotations["injected"]; exists {
		t.Error("mutating workload annotations affected original ingress annotations")
	}
}
