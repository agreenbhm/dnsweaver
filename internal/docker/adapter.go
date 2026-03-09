package docker

import (
	"context"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/workload"
)

// toWorkloadKind maps a Docker WorkloadType to the platform-agnostic workload.Kind.
func toWorkloadKind(t WorkloadType) workload.Kind {
	switch t {
	case WorkloadTypeService:
		return workload.KindService
	case WorkloadTypeContainer:
		return workload.KindContainer
	default:
		return workload.KindContainer
	}
}

// ToWorkload converts a Docker Workload into a platform-agnostic workload.Workload.
func (w Workload) ToWorkload() workload.Workload {
	return workload.Workload{
		ID:       w.ID,
		Name:     w.Name,
		Labels:   w.Labels,
		Platform: workload.PlatformDocker,
		Kind:     toWorkloadKind(w.Type),
	}
}

// WorkloadListerAdapter wraps a Docker Client to implement the workload.Lister interface.
// This allows the Docker client to be used in the platform-agnostic reconciler.
type WorkloadListerAdapter struct {
	client *Client
}

// NewWorkloadListerAdapter creates a new adapter that wraps a Docker Client.
func NewWorkloadListerAdapter(c *Client) *WorkloadListerAdapter {
	return &WorkloadListerAdapter{client: c}
}

// ListWorkloads returns all Docker workloads converted to platform-agnostic workloads.
func (a *WorkloadListerAdapter) ListWorkloads(ctx context.Context) ([]workload.Workload, error) {
	dockerWorkloads, err := a.client.ListWorkloads(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]workload.Workload, len(dockerWorkloads))
	for i, dw := range dockerWorkloads {
		result[i] = dw.ToWorkload()
	}
	return result, nil
}

// Platform returns docker, since this adapter wraps a Docker client.
func (a *WorkloadListerAdapter) Platform() workload.Platform {
	return workload.PlatformDocker
}
