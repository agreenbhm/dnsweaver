package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/maxfield-allison/dnsweaver/pkg/workload"
)

func TestConvertService(t *testing.T) {
	tests := []struct {
		name          string
		service       *corev1.Service
		wantHostnames []string
		wantName      string
		wantKind      workload.Kind
	}{
		{
			name: "external-dns annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("svc-1"),
					Name:      "my-db",
					Namespace: "databases",
					Annotations: map[string]string{
						AnnotationExternalDNSHostname: "db.example.com",
					},
				},
				Spec: corev1.ServiceSpec{
					Type:      corev1.ServiceTypeClusterIP,
					ClusterIP: "10.96.100.1",
				},
			},
			wantHostnames: []string{"db.example.com"},
			wantName:      "databases/my-db",
			wantKind:      workload.KindK8sService,
		},
		{
			name: "dnsweaver annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("svc-2"),
					Name:      "my-api",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationDNSWeaverHostname:  "api.example.com",
						AnnotationDNSWeaverHostnames: "api2.example.com,api3.example.com",
					},
				},
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
			},
			wantHostnames: []string{"api.example.com", "api2.example.com", "api3.example.com"},
			wantName:      "default/my-api",
		},
		{
			name: "comma-separated external-dns",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("svc-3"),
					Name:      "multi",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationExternalDNSHostname: "a.example.com, b.example.com",
					},
				},
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
			},
			wantHostnames: []string{"a.example.com", "b.example.com"},
		},
		{
			name: "deduplicates across annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("svc-4"),
					Name:      "dedup",
					Namespace: "default",
					Annotations: map[string]string{
						AnnotationExternalDNSHostname: "shared.example.com",
						AnnotationDNSWeaverHostname:   "shared.example.com",
					},
				},
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
			},
			wantHostnames: []string{"shared.example.com"},
		},
		{
			name: "no annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					UID:       types.UID("svc-5"),
					Name:      "plain",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
			},
			wantHostnames: nil,
		},
		{
			name: "metadata includes service type and clusterIP",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					UID:             types.UID("svc-6"),
					Name:            "lb-svc",
					Namespace:       "ingress",
					ResourceVersion: "456",
				},
				Spec: corev1.ServiceSpec{
					Type:      corev1.ServiceTypeLoadBalancer,
					ClusterIP: "10.96.200.5",
				},
			},
			wantHostnames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := ConvertService(tt.service)

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
			if w.Kind != workload.KindK8sService {
				t.Errorf("kind = %q, want k8s-service", w.Kind)
			}

			// Check metadata for the service with explicit fields
			if tt.name == "metadata includes service type and clusterIP" {
				if w.Metadata["type"] != string(corev1.ServiceTypeLoadBalancer) {
					t.Errorf("metadata.type = %q, want %q", w.Metadata["type"], corev1.ServiceTypeLoadBalancer)
				}
				if w.Metadata["clusterIP"] != "10.96.200.5" {
					t.Errorf("metadata.clusterIP = %q, want %q", w.Metadata["clusterIP"], "10.96.200.5")
				}
				if w.Metadata["resourceVersion"] != "456" {
					t.Errorf("metadata.resourceVersion = %q, want %q", w.Metadata["resourceVersion"], "456")
				}
			}
		})
	}
}
