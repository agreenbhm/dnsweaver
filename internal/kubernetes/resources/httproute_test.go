package resources

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/workload"
)

func TestConvertHTTPRoute(t *testing.T) {
	tests := []struct {
		name          string
		obj           *unstructured.Unstructured
		wantHostnames []string
		wantName      string
		wantKind      workload.Kind
	}{
		{
			name: "single hostname",
			obj: makeUnstructured("gateway.networking.k8s.io/v1", "HTTPRoute", "default", "my-route", "hr-1", map[string]interface{}{
				"hostnames": []interface{}{"myapp.example.com"},
			}),
			wantHostnames: []string{"myapp.example.com"},
			wantName:      "default/my-route",
			wantKind:      workload.KindHTTPRoute,
		},
		{
			name: "multiple hostnames",
			obj: makeUnstructured("gateway.networking.k8s.io/v1", "HTTPRoute", "web", "multi", "hr-2", map[string]interface{}{
				"hostnames": []interface{}{"app.example.com", "api.example.com"},
			}),
			wantHostnames: []string{"app.example.com", "api.example.com"},
			wantName:      "web/multi",
		},
		{
			name: "duplicate hostnames deduplicated",
			obj: makeUnstructured("gateway.networking.k8s.io/v1", "HTTPRoute", "default", "dedup", "hr-3", map[string]interface{}{
				"hostnames": []interface{}{"same.example.com", "same.example.com"},
			}),
			wantHostnames: []string{"same.example.com"},
		},
		{
			name:          "no hostnames",
			obj:           makeUnstructured("gateway.networking.k8s.io/v1", "HTTPRoute", "default", "empty", "hr-4", map[string]interface{}{}),
			wantHostnames: nil,
		},
		{
			name: "empty hostname string skipped",
			obj: makeUnstructured("gateway.networking.k8s.io/v1", "HTTPRoute", "default", "empty-str", "hr-5", map[string]interface{}{
				"hostnames": []interface{}{"", "real.example.com"},
			}),
			wantHostnames: []string{"real.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := ConvertHTTPRoute(tt.obj)

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
			if w.Kind != workload.KindHTTPRoute {
				t.Errorf("kind = %q, want httproute", w.Kind)
			}
		})
	}
}
