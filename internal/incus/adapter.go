package incus

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/maxfield-allison/dnsweaver/pkg/workload"
)

const (
	instanceTypeContainer = "container"
	instanceTypeVM        = "virtual-machine"
)

// AdapterConfig holds filtering options for the WorkloadListerAdapter.
type AdapterConfig struct {
	// StateFilter restricts listing to instances in the given state. Typical
	// values: "running", "stopped". Matching is case-insensitive. Defaults to
	// "running" if empty.
	StateFilter string
}

// WorkloadListerAdapter wraps an Incus Client to implement the workload.Lister
// interface. It lists instances (containers and VMs), applies configured
// filters, resolves IP addresses from the instance state, and converts each
// instance to a platform-agnostic workload.Workload.
type WorkloadListerAdapter struct {
	client *Client
	cfg    AdapterConfig
	logger *slog.Logger
}

// NewWorkloadListerAdapter creates a new adapter that wraps an Incus Client.
func NewWorkloadListerAdapter(c *Client, cfg AdapterConfig, logger *slog.Logger) *WorkloadListerAdapter {
	stateFilter := cfg.StateFilter
	if stateFilter == "" {
		stateFilter = "running"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &WorkloadListerAdapter{
		client: c,
		cfg:    AdapterConfig{StateFilter: stateFilter},
		logger: logger,
	}
}

// ListWorkloads returns Incus instances as platform-agnostic workloads. Applies
// the state filter and resolves each instance's IP address from its runtime
// network state.
func (a *WorkloadListerAdapter) ListWorkloads(ctx context.Context) ([]workload.Workload, error) {
	instances, err := a.client.ListInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing incus instances: %w", err)
	}

	var result []workload.Workload

	for _, inst := range instances {
		if !a.matchesFilters(inst) {
			continue
		}

		ip := ResolveIP(inst)
		if ip == "" {
			a.logger.Debug("no routable IPv4 resolved for incus instance",
				slog.String("name", inst.Name),
				slog.String("project", inst.Project),
				slog.String("type", inst.Type),
			)
		}

		result = append(result, toWorkload(inst, ip))
	}

	return result, nil
}

// Platform returns PlatformIncus, identifying this adapter as an Incus source.
func (a *WorkloadListerAdapter) Platform() workload.Platform {
	return workload.PlatformIncus
}

// matchesFilters returns true if the given instance passes all configured filters.
func (a *WorkloadListerAdapter) matchesFilters(inst Instance) bool {
	if a.cfg.StateFilter != "" && !strings.EqualFold(inst.Status, a.cfg.StateFilter) {
		return false
	}
	return true
}

// toWorkload converts an Instance and its resolved IP into a platform-agnostic
// Workload. Instance config keys (including user.* keys) are exposed as labels
// so sources can act on them.
func toWorkload(inst Instance, ip string) workload.Workload {
	kind := workload.KindIncusVM
	if inst.Type == instanceTypeContainer {
		kind = workload.KindIncusContainer
	}

	project := inst.Project
	if project == "" {
		project = "default"
	}

	meta := map[string]string{
		"project": project,
		"type":    inst.Type,
		"status":  inst.Status,
	}
	if ip != "" {
		meta["ip"] = ip
	}

	// Expose instance config keys directly as labels so sources can read
	// operator-supplied keys such as "user.dnsweaver.hostname".
	labels := make(map[string]string, len(inst.Config))
	for k, v := range inst.Config {
		labels[k] = v
	}

	return workload.Workload{
		ID:       fmt.Sprintf("%s/%s", project, inst.Name),
		Name:     inst.Name,
		Labels:   labels,
		Platform: workload.PlatformIncus,
		Kind:     kind,
		Metadata: meta,
	}
}
