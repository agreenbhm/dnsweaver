package workload

import (
	"testing"
)

func TestPlatform_String(t *testing.T) {
	tests := []struct {
		platform Platform
		want     string
	}{
		{PlatformDocker, "docker"},
		{PlatformKubernetes, "kubernetes"},
		{PlatformStatic, "static"},
	}

	for _, tt := range tests {
		if got := tt.platform.String(); got != tt.want {
			t.Errorf("Platform(%q).String() = %q, want %q", tt.platform, got, tt.want)
		}
	}
}

func TestKind_String(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindContainer, "container"},
		{KindService, "service"},
		{KindIngress, "ingress"},
		{KindIngressRoute, "ingressroute"},
		{KindHTTPRoute, "httproute"},
		{KindK8sService, "k8s-service"},
		{KindPod, "pod"},
	}

	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("Kind(%q).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestWorkload_String(t *testing.T) {
	w := Workload{
		Name:     "my-container",
		Platform: PlatformDocker,
		Kind:     KindContainer,
	}

	want := "docker/container:my-container"
	if got := w.String(); got != want {
		t.Errorf("Workload.String() = %q, want %q", got, want)
	}
}

func TestWorkload_Labels(t *testing.T) {
	w := Workload{
		Labels: map[string]string{
			"traefik.http.routers.app.rule": "Host(`app.example.com`)",
			"dnsweaver.hostname":            "app.example.com",
		},
	}

	// HasLabel
	if !w.HasLabel("dnsweaver.hostname") {
		t.Error("HasLabel(dnsweaver.hostname) = false, want true")
	}
	if w.HasLabel("nonexistent") {
		t.Error("HasLabel(nonexistent) = true, want false")
	}

	// GetLabel
	if got := w.GetLabel("dnsweaver.hostname"); got != "app.example.com" {
		t.Errorf("GetLabel() = %q, want %q", got, "app.example.com")
	}
	if got := w.GetLabel("nonexistent"); got != "" {
		t.Errorf("GetLabel(nonexistent) = %q, want empty", got)
	}

	// GetLabelOr
	if got := w.GetLabelOr("nonexistent", "default"); got != "default" {
		t.Errorf("GetLabelOr(nonexistent) = %q, want %q", got, "default")
	}
	if got := w.GetLabelOr("dnsweaver.hostname", "default"); got != "app.example.com" {
		t.Errorf("GetLabelOr(existing) = %q, want %q", got, "app.example.com")
	}
}

func TestWorkload_Labels_NilMap(t *testing.T) {
	w := Workload{}

	if w.HasLabel("any") {
		t.Error("HasLabel on nil labels = true, want false")
	}
	if got := w.GetLabel("any"); got != "" {
		t.Errorf("GetLabel on nil labels = %q, want empty", got)
	}
	if got := w.GetLabelOr("any", "default"); got != "default" {
		t.Errorf("GetLabelOr on nil labels = %q, want %q", got, "default")
	}
}

func TestWorkload_Annotations(t *testing.T) {
	w := Workload{
		Annotations: map[string]string{
			"dnsweaver.dev/hostname": "app.example.com",
		},
	}

	if !w.HasAnnotation("dnsweaver.dev/hostname") {
		t.Error("HasAnnotation(dnsweaver.dev/hostname) = false, want true")
	}
	if w.HasAnnotation("nonexistent") {
		t.Error("HasAnnotation(nonexistent) = true, want false")
	}

	if got := w.GetAnnotation("dnsweaver.dev/hostname"); got != "app.example.com" {
		t.Errorf("GetAnnotation() = %q, want %q", got, "app.example.com")
	}
	if got := w.GetAnnotation("nonexistent"); got != "" {
		t.Errorf("GetAnnotation(nonexistent) = %q, want empty", got)
	}

	if got := w.GetAnnotationOr("nonexistent", "default"); got != "default" {
		t.Errorf("GetAnnotationOr(nonexistent) = %q, want %q", got, "default")
	}
}

func TestWorkload_Annotations_NilMap(t *testing.T) {
	w := Workload{}

	if w.HasAnnotation("any") {
		t.Error("HasAnnotation on nil annotations = true, want false")
	}
	if got := w.GetAnnotation("any"); got != "" {
		t.Errorf("GetAnnotation on nil annotations = %q, want empty", got)
	}
	if got := w.GetAnnotationOr("any", "default"); got != "default" {
		t.Errorf("GetAnnotationOr on nil annotations = %q, want %q", got, "default")
	}
}

func TestWorkload_GetLabelOrAnnotation(t *testing.T) {
	w := Workload{
		Labels: map[string]string{
			"shared-key": "from-label",
		},
		Annotations: map[string]string{
			"shared-key":      "from-annotation",
			"annotation-only": "annotation-value",
		},
	}

	// Labels take priority
	if got := w.GetLabelOrAnnotation("shared-key"); got != "from-label" {
		t.Errorf("GetLabelOrAnnotation(shared-key) = %q, want %q", got, "from-label")
	}

	// Falls back to annotation
	if got := w.GetLabelOrAnnotation("annotation-only"); got != "annotation-value" {
		t.Errorf("GetLabelOrAnnotation(annotation-only) = %q, want %q", got, "annotation-value")
	}

	// Not found
	if got := w.GetLabelOrAnnotation("nonexistent"); got != "" {
		t.Errorf("GetLabelOrAnnotation(nonexistent) = %q, want empty", got)
	}
}

func TestWorkload_GetLabelOrAnnotation_NilMaps(t *testing.T) {
	w := Workload{}

	if got := w.GetLabelOrAnnotation("any"); got != "" {
		t.Errorf("GetLabelOrAnnotation on nil maps = %q, want empty", got)
	}
}

func TestWorkload_IsDocker(t *testing.T) {
	docker := Workload{Platform: PlatformDocker}
	k8s := Workload{Platform: PlatformKubernetes}

	if !docker.IsDocker() {
		t.Error("Docker workload.IsDocker() = false, want true")
	}
	if docker.IsKubernetes() {
		t.Error("Docker workload.IsKubernetes() = true, want false")
	}
	if k8s.IsDocker() {
		t.Error("K8s workload.IsDocker() = true, want false")
	}
	if !k8s.IsKubernetes() {
		t.Error("K8s workload.IsKubernetes() = false, want true")
	}
}

func TestWorkload_DockerLikeUsage(t *testing.T) {
	// Ensure a Docker-like workload works the same as the old docker.Workload
	w := Workload{
		ID:       "abc123",
		Name:     "my-container",
		Platform: PlatformDocker,
		Kind:     KindContainer,
		Labels: map[string]string{
			"traefik.http.routers.app.rule": "Host(`app.example.com`)",
		},
	}

	if w.ID != "abc123" {
		t.Errorf("ID = %q, want %q", w.ID, "abc123")
	}
	if w.Name != "my-container" {
		t.Errorf("Name = %q, want %q", w.Name, "my-container")
	}
	if !w.HasLabel("traefik.http.routers.app.rule") {
		t.Error("should have traefik label")
	}
	if w.Namespace != "" {
		t.Error("Docker workload should have empty namespace")
	}
}

func TestWorkload_KubernetesLikeUsage(t *testing.T) {
	w := Workload{
		ID:        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		Name:      "default/my-ingress",
		Namespace: "default",
		Platform:  PlatformKubernetes,
		Kind:      KindIngress,
		Labels: map[string]string{
			"app.kubernetes.io/name": "my-app",
		},
		Annotations: map[string]string{
			"dnsweaver.dev/record-type": "A",
			"dnsweaver.dev/target":      "10.30.0.100",
		},
		Hostnames: []string{"myapp.example.com", "www.example.com"},
		Metadata: map[string]string{
			"resourceVersion": "12345",
		},
	}

	if w.Namespace != "default" {
		t.Errorf("Namespace = %q, want %q", w.Namespace, "default")
	}
	if len(w.Hostnames) != 2 {
		t.Errorf("Hostnames = %d, want 2", len(w.Hostnames))
	}
	if w.GetAnnotation("dnsweaver.dev/record-type") != "A" {
		t.Error("should have annotation dnsweaver.dev/record-type=A")
	}
	if w.Metadata["resourceVersion"] != "12345" {
		t.Error("should have metadata resourceVersion")
	}
}
