package docker

import (
	"testing"

	"github.com/maxfield-allison/dnsweaver/pkg/workload"
)

func TestWorkload_ToWorkload_Service(t *testing.T) {
	dw := Workload{
		ID:   "svc-123",
		Name: "my-service",
		Labels: map[string]string{
			"traefik.enable": "true",
		},
		Type: WorkloadTypeService,
	}

	w := dw.ToWorkload()

	if w.ID != "svc-123" {
		t.Errorf("ID = %q, want %q", w.ID, "svc-123")
	}
	if w.Name != "my-service" {
		t.Errorf("Name = %q, want %q", w.Name, "my-service")
	}
	if w.Platform != workload.PlatformDocker {
		t.Errorf("Platform = %q, want %q", w.Platform, workload.PlatformDocker)
	}
	if w.Kind != workload.KindService {
		t.Errorf("Kind = %q, want %q", w.Kind, workload.KindService)
	}
	if w.Labels["traefik.enable"] != "true" {
		t.Errorf("Labels[traefik.enable] = %q, want %q", w.Labels["traefik.enable"], "true")
	}
}

func TestWorkload_ToWorkload_Container(t *testing.T) {
	dw := Workload{
		ID:     "ctr-456",
		Name:   "my-container",
		Labels: map[string]string{},
		Type:   WorkloadTypeContainer,
	}

	w := dw.ToWorkload()

	if w.Platform != workload.PlatformDocker {
		t.Errorf("Platform = %q, want %q", w.Platform, workload.PlatformDocker)
	}
	if w.Kind != workload.KindContainer {
		t.Errorf("Kind = %q, want %q", w.Kind, workload.KindContainer)
	}
}

func TestWorkload_ToWorkload_NilLabels(t *testing.T) {
	dw := Workload{
		ID:   "ctr-789",
		Name: "no-labels",
		Type: WorkloadTypeContainer,
	}

	w := dw.ToWorkload()

	if w.Labels != nil {
		t.Errorf("Labels = %v, want nil (preserved from Docker workload)", w.Labels)
	}
}

func TestToWorkloadKind(t *testing.T) {
	tests := []struct {
		input WorkloadType
		want  workload.Kind
	}{
		{WorkloadTypeService, workload.KindService},
		{WorkloadTypeContainer, workload.KindContainer},
		{WorkloadType("unknown"), workload.KindContainer},
	}

	for _, tt := range tests {
		got := toWorkloadKind(tt.input)
		if got != tt.want {
			t.Errorf("toWorkloadKind(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWorkloadListerAdapter_Platform(t *testing.T) {
	adapter := NewWorkloadListerAdapter(nil)

	if adapter.Platform() != workload.PlatformDocker {
		t.Errorf("Platform() = %q, want %q", adapter.Platform(), workload.PlatformDocker)
	}
}
