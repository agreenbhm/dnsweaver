package ovh

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // mirrors the signature scheme under test
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

const (
	testAppKey    = "app-key"
	testAppSecret = "app-secret"
	testConsumer  = "consumer-key"
	testServerTS  = 1700000000
)

// newTestClient returns a client pointed at the given server URL.
func newTestClient(serverURL string) *Client {
	return NewClient(serverURL, testAppKey, testAppSecret, testConsumer)
}

// ovhMux builds a test server that always answers /auth/time and dispatches the
// rest to the provided handler. It also asserts the signature headers are
// present and correct for non-time requests.
func ovhMux(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/time" {
			fmt.Fprintf(w, "%d", testServerTS)
			return
		}

		// Verify auth headers.
		if got := r.Header.Get("X-Ovh-Application"); got != testAppKey {
			t.Errorf("X-Ovh-Application = %q, want %q", got, testAppKey)
		}
		if got := r.Header.Get("X-Ovh-Consumer"); got != testConsumer {
			t.Errorf("X-Ovh-Consumer = %q, want %q", got, testConsumer)
		}
		ts := r.Header.Get("X-Ovh-Timestamp")
		if ts == "" {
			t.Error("missing X-Ovh-Timestamp header")
		}
		tsInt, _ := strconv.ParseInt(ts, 10, 64)

		// Read the body once for signature verification, then restore it so
		// the downstream handler can decode it.
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Recompute expected signature.
		fullURL := srv.URL + r.URL.RequestURI()
		h := sha1.New() //nolint:gosec
		fmt.Fprintf(h, "%s+%s+%s+%s+%s+%d", testAppSecret, testConsumer, r.Method, fullURL, string(bodyBytes), tsInt)
		want := "$1$" + fmt.Sprintf("%x", h.Sum(nil))
		if got := r.Header.Get("X-Ovh-Signature"); got != want {
			t.Errorf("X-Ovh-Signature = %q, want %q", got, want)
		}

		handler(w, r)
	}))
	return srv
}

func TestClient_TimeDeltaAndSigning(t *testing.T) {
	// Pin local time so the delta is deterministic.
	orig := nowFunc
	nowFunc = func() int64 { return testServerTS - 5 }
	defer func() { nowFunc = orig }()

	srv := ovhMux(t, func(w http.ResponseWriter, r *http.Request) {
		// Timestamp should be local + delta == server time.
		if got := r.Header.Get("X-Ovh-Timestamp"); got != strconv.Itoa(testServerTS) {
			t.Errorf("timestamp = %s, want %d", got, testServerTS)
		}
		_, _ = w.Write([]byte(`{"id":1,"zone":"example.com","subDomain":"app","fieldType":"A","target":"10.0.0.1","ttl":3600}`))
	})
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.GetRecord(context.Background(), "example.com", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Ping(t *testing.T) {
	srv := ovhMux(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/domain/zone/example.com/soa" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"server":"dns.ovh.net","ttl":3600}`))
	})
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.Ping(context.Background(), "example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Ping_Error(t *testing.T) {
	srv := ovhMux(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"This call has not been granted","errorCode":"NOT_GRANTED_CALL"}`))
	})
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.Ping(context.Background(), "example.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClient_ListRecordIDs(t *testing.T) {
	srv := ovhMux(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fieldType") != "A" {
			t.Errorf("expected fieldType=A, got %s", r.URL.Query().Get("fieldType"))
		}
		if r.URL.Query().Get("subDomain") != "app" {
			t.Errorf("expected subDomain=app, got %s", r.URL.Query().Get("subDomain"))
		}
		_, _ = w.Write([]byte(`[101,102]`))
	})
	defer srv.Close()

	c := newTestClient(srv.URL)
	ids, err := c.ListRecordIDs(context.Background(), "example.com", "A", "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != 101 || ids[1] != 102 {
		t.Errorf("unexpected ids: %v", ids)
	}
}

func TestClient_CreateRecord(t *testing.T) {
	srv := ovhMux(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req recordCreateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.FieldType != "A" || req.SubDomain != "app" || req.Target != "10.0.0.1" {
			t.Errorf("unexpected request body: %+v", req)
		}
		_, _ = w.Write([]byte(`{"id":555,"zone":"example.com","subDomain":"app","fieldType":"A","target":"10.0.0.1","ttl":3600}`))
	})
	defer srv.Close()

	c := newTestClient(srv.URL)
	rec, err := c.CreateRecord(context.Background(), "example.com", "A", "app", "10.0.0.1", 3600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.ID != 555 {
		t.Errorf("expected id 555, got %d", rec.ID)
	}
}

func TestClient_UpdateRecord(t *testing.T) {
	srv := ovhMux(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/domain/zone/example.com/record/555" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.UpdateRecord(context.Background(), "example.com", 555, "app", "10.0.0.2", 3600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteRecord(t *testing.T) {
	srv := ovhMux(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/domain/zone/example.com/record/555" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.DeleteRecord(context.Background(), "example.com", 555); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_RefreshZone(t *testing.T) {
	called := false
	srv := ovhMux(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/domain/zone/example.com/refresh" && r.Method == http.MethodPost {
			called = true
		}
		w.WriteHeader(http.StatusOK)
	})
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.RefreshZone(context.Background(), "example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected refresh endpoint to be called")
	}
}
