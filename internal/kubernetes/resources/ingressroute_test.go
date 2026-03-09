package resources

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/workload"
)

func TestConvertIngressRoute(t *testing.T) {
	tests := []struct {
		name          string
		obj           *unstructured.Unstructured
		wantHostnames []string
		wantName      string
		wantKind      workload.Kind
	}{
		{
			name: "single host matcher",
			obj: makeUnstructured("traefik.io/v1alpha1", "IngressRoute", "default", "my-app", "ir-1", map[string]interface{}{
				"routes": []interface{}{
					map[string]interface{}{
						"match": "Host(`myapp.example.com`)",
						"kind":  "Rule",
					},
				},
			}),
			wantHostnames: []string{"myapp.example.com"},
			wantName:      "default/my-app",
			wantKind:      workload.KindIngressRoute,
		},
		{
			name: "multiple hosts in one route",
			obj: makeUnstructured("traefik.io/v1alpha1", "IngressRoute", "web", "multi", "ir-2", map[string]interface{}{
				"routes": []interface{}{
					map[string]interface{}{
						"match": "Host(`a.example.com`) || Host(`b.example.com`)",
					},
				},
			}),
			wantHostnames: []string{"a.example.com", "b.example.com"},
			wantName:      "web/multi",
		},
		{
			name: "host with path prefix",
			obj: makeUnstructured("traefik.io/v1alpha1", "IngressRoute", "default", "with-path", "ir-3", map[string]interface{}{
				"routes": []interface{}{
					map[string]interface{}{
						"match": "Host(`api.example.com`) && PathPrefix(`/v1`)",
					},
				},
			}),
			wantHostnames: []string{"api.example.com"},
		},
		{
			name: "multiple routes",
			obj: makeUnstructured("traefik.io/v1alpha1", "IngressRoute", "default", "multi-route", "ir-4", map[string]interface{}{
				"routes": []interface{}{
					map[string]interface{}{
						"match": "Host(`web.example.com`)",
					},
					map[string]interface{}{
						"match": "Host(`api.example.com`) && PathPrefix(`/api`)",
					},
				},
			}),
			wantHostnames: []string{"web.example.com", "api.example.com"},
		},
		{
			name: "duplicate hosts deduplicated",
			obj: makeUnstructured("traefik.io/v1alpha1", "IngressRoute", "default", "dedup", "ir-5", map[string]interface{}{
				"routes": []interface{}{
					map[string]interface{}{
						"match": "Host(`same.example.com`)",
					},
					map[string]interface{}{
						"match": "Host(`same.example.com`) && PathPrefix(`/api`)",
					},
				},
			}),
			wantHostnames: []string{"same.example.com"},
		},
		{
			name: "no routes",
			obj: makeUnstructured("traefik.io/v1alpha1", "IngressRoute", "default", "empty", "ir-6", map[string]interface{}{
				"routes": []interface{}{},
			}),
			wantHostnames: nil,
		},
		{
			name: "no match field",
			obj: makeUnstructured("traefik.io/v1alpha1", "IngressRoute", "default", "no-match", "ir-7", map[string]interface{}{
				"routes": []interface{}{
					map[string]interface{}{
						"kind": "Rule",
					},
				},
			}),
			wantHostnames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := ConvertIngressRoute(tt.obj)

			if len(w.Hostnames) != len(tt.wantHostnames) {
				t.Errorf("hostnames count = %d, want %d; got %v", len(w.Hostnames), len(tt.wantHostnames), w.Hostnames)
			}
			for i, want := range tt.wantHostnames {
				if i < len(w.Hostnames) && w.Hostnames[i] != want {
					t.Errorf("hostname[%d] = %q, want %q", i, w.Hostnames[i], want)
				}
			}

			if tt.wantName != "" && w.Name != tt.wantName {
				t.Errorf("name = %q, want %q", w.Name, tt.wantName)
			}
			if w.Platform != workload.PlatformKubernetes {
				t.Errorf("platform = %q, want kubernetes", w.Platform)
			}
			if w.Kind != workload.KindIngressRoute {
				t.Errorf("kind = %q, want ingressroute", w.Kind)
			}
		})
	}
}

func TestExtractHostsFromMatch(t *testing.T) {
	tests := []struct {
		match string
		want  []string
	}{
		{"Host(`example.com`)", []string{"example.com"}},
		{"Host(`a.com`) || Host(`b.com`)", []string{"a.com", "b.com"}},
		{"Host(`a.com`) && PathPrefix(`/api`)", []string{"a.com"}},
		{"(Host(`a.com`) || Host(`b.com`)) && PathPrefix(`/`)", []string{"a.com", "b.com"}},
		{"PathPrefix(`/health`)", nil},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.match, func(t *testing.T) {
			got := extractHostsFromMatch(tt.match)
			if len(got) != len(tt.want) {
				t.Errorf("extractHostsFromMatch(%q) = %v, want %v", tt.match, got, tt.want)
				return
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("host[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// makeUnstructured creates an unstructured K8s object for testing.
func makeUnstructured(apiVersion, kind, namespace, name, uid string, spec map[string]interface{}) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":            name,
				"namespace":       namespace,
				"uid":             uid,
				"resourceVersion": "1",
			},
			"spec": spec,
		},
	}
	return obj
}
