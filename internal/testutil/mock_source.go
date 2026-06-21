package testutil

import (
	"context"

	"github.com/maxfield-allison/dnsweaver/pkg/source"
	"github.com/maxfield-allison/dnsweaver/pkg/workload"
)

// MockSource implements source.Source for testing.
type MockSource struct {
	name       string
	hostnames  []source.Hostname
	extractFn  func(ctx context.Context, w workload.Workload) ([]source.Hostname, error)
	discoverFn func(ctx context.Context) ([]source.Hostname, error)
	discovery  bool
	platforms  []workload.Platform
}

// NewMockSource creates a MockSource that returns the given hostnames on Extract.
func NewMockSource(name string, hostnames ...source.Hostname) *MockSource {
	return &MockSource{
		name:      name,
		hostnames: hostnames,
	}
}

// Name returns the source identifier.
func (m *MockSource) Name() string { return m.name }

// Extract returns configured hostnames or calls the custom extractFn.
func (m *MockSource) Extract(ctx context.Context, w workload.Workload) ([]source.Hostname, error) {
	if m.extractFn != nil {
		return m.extractFn(ctx, w)
	}
	return m.hostnames, nil
}

// Discover returns hostnames from file discovery or calls custom discoverFn.
func (m *MockSource) Discover(ctx context.Context) ([]source.Hostname, error) {
	if m.discoverFn != nil {
		return m.discoverFn(ctx)
	}
	return nil, nil
}

// SupportsDiscovery returns whether file-based discovery is enabled.
func (m *MockSource) SupportsDiscovery() bool {
	return m.discovery
}

// SupportedPlatforms returns which platforms this source handles.
// Empty means all platforms.
func (m *MockSource) SupportedPlatforms() []workload.Platform {
	return m.platforms
}

// --- Configuration methods ---

// SetHostnames sets the hostnames returned by Extract.
func (m *MockSource) SetHostnames(hostnames ...source.Hostname) {
	m.hostnames = hostnames
}

// SetExtractFunc configures a custom function called on Extract.
func (m *MockSource) SetExtractFunc(fn func(ctx context.Context, w workload.Workload) ([]source.Hostname, error)) {
	m.extractFn = fn
}

// SetDiscoverFunc configures a custom function called on Discover.
// Also enables discovery.
func (m *MockSource) SetDiscoverFunc(fn func(ctx context.Context) ([]source.Hostname, error)) {
	m.discoverFn = fn
	m.discovery = true
}

// SetSupportsDiscovery configures whether this source supports file discovery.
func (m *MockSource) SetSupportsDiscovery(enabled bool) {
	m.discovery = enabled
}

// SetPlatforms configures which platforms this source handles.
func (m *MockSource) SetPlatforms(platforms ...workload.Platform) {
	m.platforms = platforms
}

// Compile-time interface check.
var _ source.Source = (*MockSource)(nil)
