package testutil_test

import (
	"context"
	"net/http"
	"testing"

	"gitlab.bluewillows.net/root/dnsweaver/internal/testutil"
)

// httpGet is a test helper that performs an HTTP GET with context.
func httpGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	testutil.RequireNoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	testutil.RequireNoError(t, err)
	return resp
}

func TestMockServer_BasicRouting(t *testing.T) {
	srv := testutil.NewMockServer(t)

	srv.Handle("/api/ping", testutil.JSONResponse(200, map[string]string{"status": "ok"}))
	srv.Handle("/api/error", testutil.ErrorResponse(500, "internal error"))

	// Hit ping
	resp := httpGet(t, srv.URL+"/api/ping")
	testutil.AssertEqual(t, "ping status", resp.StatusCode, 200)
	resp.Body.Close()

	// Hit error
	resp = httpGet(t, srv.URL+"/api/error")
	testutil.AssertEqual(t, "error status", resp.StatusCode, 500)
	resp.Body.Close()

	// Hit unknown path → 404
	resp = httpGet(t, srv.URL+"/unknown")
	testutil.AssertEqual(t, "unknown status", resp.StatusCode, 404)
	resp.Body.Close()
}

func TestMockServer_RequestRecording(t *testing.T) {
	srv := testutil.NewMockServer(t)
	srv.HandleDefault(testutil.EmptyResponse(200))

	resp := httpGet(t, srv.URL+"/api/test?token=abc")
	resp.Body.Close()

	reqs := srv.Requests()
	testutil.AssertLen(t, "recorded requests", reqs, 1)
	testutil.AssertEqual(t, "method", reqs[0].Method, "GET")
	testutil.AssertEqual(t, "path", reqs[0].Path, "/api/test")
	testutil.AssertContains(t, reqs[0].Query, "token=abc")
}

func TestMockServer_RequestsForPath(t *testing.T) {
	srv := testutil.NewMockServer(t)
	srv.HandleDefault(testutil.EmptyResponse(200))

	// Make requests to different paths
	for _, path := range []string{"/a", "/b", "/a", "/c", "/a"} {
		resp := httpGet(t, srv.URL+path)
		resp.Body.Close()
	}

	testutil.AssertLen(t, "path /a requests", srv.RequestsForPath("/a"), 3)
	testutil.AssertLen(t, "path /b requests", srv.RequestsForPath("/b"), 1)
	testutil.AssertEqual(t, "total requests", srv.RequestCount(), 5)
}

func TestMockServer_ResetRequests(t *testing.T) {
	srv := testutil.NewMockServer(t)
	srv.HandleDefault(testutil.EmptyResponse(200))

	resp := httpGet(t, srv.URL+"/test")
	resp.Body.Close()
	testutil.AssertEqual(t, "count before reset", srv.RequestCount(), 1)

	srv.ResetRequests()
	testutil.AssertEqual(t, "count after reset", srv.RequestCount(), 0)
}

func TestMockSource_BasicBehavior(t *testing.T) {
	ctx := context.Background()
	h := testutil.Hostname("app.example.com", "traefik")

	src := testutil.NewMockSource("traefik", h)
	testutil.AssertEqual(t, "name", src.Name(), "traefik")

	hostnames, err := src.Extract(ctx, testutil.DockerWorkload("myapp", nil))
	testutil.RequireNoError(t, err)
	testutil.AssertLen(t, "hostnames", hostnames, 1)
	testutil.AssertEqual(t, "hostname name", hostnames[0].Name, "app.example.com")

	// Discovery defaults to disabled
	if src.SupportsDiscovery() {
		t.Error("expected discovery to be disabled by default")
	}
}

func TestMockWorkloadLister_Operations(t *testing.T) {
	ctx := context.Background()
	lister := testutil.NewMockWorkloadLister("docker")

	lister.AddWorkload("myapp", map[string]string{"traefik.host": "app.example.com"})
	lister.AddWorkload("db", map[string]string{"traefik.host": "db.example.com"})

	workloads, err := lister.ListWorkloads(ctx)
	testutil.RequireNoError(t, err)
	testutil.AssertLen(t, "workloads", workloads, 2)
}
