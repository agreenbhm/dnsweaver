package testutil

import (
	"context"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/workload"
)

// MockWorkloadLister implements workload.Lister for testing.
type MockWorkloadLister struct {
	platform  workload.Platform
	workloads []workload.Workload
	listErr   error
}

// NewMockWorkloadLister creates a workload lister for the given platform.
func NewMockWorkloadLister(platform workload.Platform) *MockWorkloadLister {
	return &MockWorkloadLister{
		platform:  platform,
		workloads: make([]workload.Workload, 0),
	}
}

// ListWorkloads returns all configured workloads or the configured error.
func (m *MockWorkloadLister) ListWorkloads(_ context.Context) ([]workload.Workload, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.workloads, nil
}

// Platform returns the platform this lister covers.
func (m *MockWorkloadLister) Platform() workload.Platform {
	return m.platform
}

// --- Configuration methods ---

// AddWorkload adds a workload with the given name and labels.
func (m *MockWorkloadLister) AddWorkload(name string, labels map[string]string) {
	m.workloads = append(m.workloads, workload.Workload{
		ID:       "id-" + name,
		Name:     name,
		Labels:   labels,
		Platform: m.platform,
		Kind:     workload.KindService,
	})
}

// AddWorkloadFull adds a fully configured workload.
func (m *MockWorkloadLister) AddWorkloadFull(w workload.Workload) {
	m.workloads = append(m.workloads, w)
}

// SetListError configures ListWorkloads to return the given error.
func (m *MockWorkloadLister) SetListError(err error) {
	m.listErr = err
}

// Reset clears all workloads and errors.
func (m *MockWorkloadLister) Reset() {
	m.workloads = make([]workload.Workload, 0)
	m.listErr = nil
}

// --- Workload builders ---

// DockerWorkload creates a Docker workload with the given name and labels.
func DockerWorkload(name string, labels map[string]string) workload.Workload {
	return workload.Workload{
		ID:       "docker-" + name,
		Name:     name,
		Labels:   labels,
		Platform: workload.PlatformDocker,
		Kind:     workload.KindService,
	}
}

// K8sIngressWorkload creates a Kubernetes Ingress workload.
func K8sIngressWorkload(namespace, name string, annotations map[string]string, hostnames ...string) workload.Workload {
	return workload.Workload{
		ID:          "k8s-" + namespace + "-" + name,
		Name:        namespace + "/" + name,
		Namespace:   namespace,
		Annotations: annotations,
		Hostnames:   hostnames,
		Platform:    workload.PlatformKubernetes,
		Kind:        workload.KindIngress,
	}
}

// K8sServiceWorkload creates a Kubernetes Service workload.
func K8sServiceWorkload(namespace, name string, annotations map[string]string) workload.Workload {
	return workload.Workload{
		ID:          "k8s-" + namespace + "-" + name,
		Name:        namespace + "/" + name,
		Namespace:   namespace,
		Annotations: annotations,
		Platform:    workload.PlatformKubernetes,
		Kind:        workload.KindK8sService,
	}
}

// Compile-time interface check.
var _ workload.Lister = (*MockWorkloadLister)(nil)
