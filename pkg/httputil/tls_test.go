package httputil

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// httpGet performs a context-bound GET so tests satisfy noctx + bodyclose linters.
func httpGet(t *testing.T, c *http.Client, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

// genCert creates a self-signed ECDSA leaf certificate suitable for both
// server and client authentication. Returns PEM-encoded cert + key bytes.
func genCert(t *testing.T, hosts []string, isClient bool) (certPEM, keyPEM []byte, leaf *x509.Certificate, priv *ecdsa.PrivateKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "dnsweaver-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true, // self-signed root for the test
		DNSNames:              hosts,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	if isClient {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	leaf, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalECPrivateKey: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, leaf, priv
}

func writeTemp(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// TestNewClient_TLS_CustomCA verifies that a server using a private CA is
// trusted when CAFile is configured, and rejected when it is not.
func TestNewClient_TLS_CustomCA(t *testing.T) {
	srvCertPEM, srvKeyPEM, srvLeaf, srvKey := genCert(t, []string{"127.0.0.1"}, false)
	dir := t.TempDir()
	caPath := writeTemp(t, dir, "ca.pem", srvCertPEM)

	srvTLSCert, err := tls.X509KeyPair(srvCertPEM, srvKeyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	_ = srvLeaf
	_ = srvKey

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{srvTLSCert}}
	srv.StartTLS()
	defer srv.Close()

	t.Run("trusted via CAFile", func(t *testing.T) {
		c := NewClient(&ClientConfig{
			TLS: &TLSConfig{CAFile: caPath},
		})
		resp := httpGet(t, c, srv.URL)
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("status=%d", resp.StatusCode)
		}
	})

	t.Run("rejected without CAFile", func(t *testing.T) {
		c := NewClient(nil)
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
		if err != nil {
			t.Fatalf("NewRequest: %v", err)
		}
		resp, err := c.Do(req)
		if err == nil {
			resp.Body.Close()
			t.Fatal("expected verification failure, got nil")
		}
	})
}

// TestNewClient_TLS_BadCA ensures a malformed CA bundle surfaces a clear error
// at Build time rather than silently producing an empty trust store.
func TestNewClient_TLS_BadCA(t *testing.T) {
	dir := t.TempDir()
	bad := writeTemp(t, dir, "ca.pem", []byte("not a certificate"))

	_, err := (&TLSConfig{CAFile: bad}).Build()
	if err == nil {
		t.Fatal("expected error for malformed CA bundle")
	}
}

// TestNewClient_TLS_MTLS verifies the client presents a configured client
// certificate when the server requires one.
func TestNewClient_TLS_MTLS(t *testing.T) {
	srvCertPEM, srvKeyPEM, _, _ := genCert(t, []string{"127.0.0.1"}, false)
	cliCertPEM, cliKeyPEM, cliLeaf, _ := genCert(t, []string{"127.0.0.1"}, true)

	dir := t.TempDir()
	caPath := writeTemp(t, dir, "ca.pem", srvCertPEM)
	cliCertPath := writeTemp(t, dir, "client.crt", cliCertPEM)
	cliKeyPath := writeTemp(t, dir, "client.key", cliKeyPEM)

	srvTLSCert, err := tls.X509KeyPair(srvCertPEM, srvKeyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(cliLeaf)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "no client cert", http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, "authenticated")
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{srvTLSCert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	srv.StartTLS()
	defer srv.Close()

	c := NewClient(&ClientConfig{
		TLS: &TLSConfig{
			CAFile:   caPath,
			CertFile: cliCertPath,
			KeyFile:  cliKeyPath,
		},
	})
	resp := httpGet(t, c, srv.URL)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

// TestTLSConfig_Build_MTLSHalves enforces that CertFile and KeyFile must be
// set together.
func TestTLSConfig_Build_MTLSHalves(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM, _, _ := genCert(t, []string{"x"}, true)
	certPath := writeTemp(t, dir, "c.crt", certPEM)
	keyPath := writeTemp(t, dir, "c.key", keyPEM)

	if _, err := (&TLSConfig{CertFile: certPath}).Build(); err == nil {
		t.Error("expected error when KeyFile missing")
	}
	if _, err := (&TLSConfig{KeyFile: keyPath}).Build(); err == nil {
		t.Error("expected error when CertFile missing")
	}
	if _, err := (&TLSConfig{CertFile: certPath, KeyFile: keyPath}).Build(); err != nil {
		t.Errorf("unexpected error with both set: %v", err)
	}
}

// TestTLSConfig_Build_ServerName verifies SNI override propagates into the
// generated *tls.Config.
func TestTLSConfig_Build_ServerName(t *testing.T) {
	cfg, err := (&TLSConfig{ServerName: "alt.example.com"}).Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil tls.Config")
	}
	if cfg.ServerName != "alt.example.com" {
		t.Errorf("ServerName = %q, want %q", cfg.ServerName, "alt.example.com")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %x, want %x", cfg.MinVersion, tls.VersionTLS12)
	}
}

// TestTLSConfig_IsZero covers the empty-vs-populated detection.
func TestTLSConfig_IsZero(t *testing.T) {
	if !(TLSConfig{}).IsZero() {
		t.Error("zero value should be IsZero")
	}
	cases := []TLSConfig{
		{CAFile: "x"},
		{CertFile: "x"},
		{KeyFile: "x"},
		{ServerName: "x"},
		{InsecureSkip: true},
		{MinVersion: tls.VersionTLS13},
	}
	for i, c := range cases {
		if c.IsZero() {
			t.Errorf("case %d: expected non-zero", i)
		}
	}
}

func TestParseTLSMinVersion(t *testing.T) {
	cases := []struct {
		in   string
		want uint16
		err  bool
	}{
		{"", 0, false},
		{"1.2", tls.VersionTLS12, false},
		{"1.3", tls.VersionTLS13, false},
		{"TLS1.2", tls.VersionTLS12, false},
		{"tls1.3", tls.VersionTLS13, false},
		{"v1.2", tls.VersionTLS12, false},
		{"  1.3  ", tls.VersionTLS13, false},
		{"1.1", 0, true},
		{"garbage", 0, true},
	}
	for _, c := range cases {
		got, err := ParseTLSMinVersion(c.in)
		if c.err {
			if err == nil {
				t.Errorf("%q: expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("%q: got %x, want %x", c.in, got, c.want)
		}
	}
}

// TestNewClient_TLSSkipVerify_PreservesTransportClone is a regression test for
// the bug where enabling InsecureSkip discarded http.DefaultTransport's
// HTTP/2, proxy, and pool settings by constructing a bare http.Transport.
// After the fix, the transport must still come from a clone of
// http.DefaultTransport (preserving non-zero MaxIdleConns).
func TestNewClient_TLSSkipVerify_PreservesTransportClone(t *testing.T) {
	c := NewClient(&ClientConfig{TLS: &TLSConfig{InsecureSkip: true}})
	uat, ok := c.Transport.(*userAgentTransport)
	if !ok {
		t.Fatalf("transport type = %T", c.Transport)
	}
	tr, ok := uat.base.(*http.Transport)
	if !ok {
		t.Fatalf("base transport type = %T", uat.base)
	}
	if tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify not set on TLSClientConfig")
	}
	if tr.MaxIdleConns == 0 {
		t.Error("MaxIdleConns is zero — transport was not cloned from DefaultTransport")
	}
}

// TestPermissionHint verifies that a permission-denied file error is annotated
// with the uid/gid the process actually runs as. This is the user-facing half
// of the fix for issue #90: the container drops privileges to the unprivileged
// dnsweaver user, so "permission denied" on a root-owned cert/key needs to spell
// out the runtime uid/gid or operators waste time thinking "but I AM root."
func TestPermissionHint(t *testing.T) {
	// A wrapped fs.ErrPermission gets the uid/gid annotation while preserving
	// the original error chain.
	base := fmt.Errorf("open /etc/certs/key.pem: %w", fs.ErrPermission)
	got := permissionHint(base)
	if got == nil {
		t.Fatal("expected non-nil error")
	}
	msg := got.Error()
	if !strings.Contains(msg, fmt.Sprintf("uid=%d", os.Getuid())) {
		t.Errorf("hint missing runtime uid: %q", msg)
	}
	if !strings.Contains(msg, fmt.Sprintf("gid=%d", os.Getgid())) {
		t.Errorf("hint missing runtime gid: %q", msg)
	}
	if !errors.Is(got, fs.ErrPermission) {
		t.Error("permissionHint must preserve the wrapped error for errors.Is")
	}

	// Non-permission errors pass through untouched (identity).
	other := errors.New("some other failure")
	if permissionHint(other) != other { //nolint:errorlint // identity check is intentional
		t.Error("non-permission error should pass through unchanged")
	}

	// nil stays nil.
	if permissionHint(nil) != nil {
		t.Error("nil should pass through unchanged")
	}
}

// TestTLSConfig_Build_KeyPermissionDenied is an end-to-end check that Build()
// routes the keypair load error through permissionHint. Skipped when running as
// root, which can read 0000-mode files and would never hit the permission path.
func TestTLSConfig_Build_KeyPermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: cannot reproduce a permission-denied read")
	}
	certPEM, keyPEM, _, _ := genCert(t, []string{"x"}, true)
	dir := t.TempDir()
	certPath := writeTemp(t, dir, "c.crt", certPEM)
	keyPath := writeTemp(t, dir, "c.key", keyPEM)
	if err := os.Chmod(keyPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	_, err := (&TLSConfig{CertFile: certPath, KeyFile: keyPath}).Build()
	if err == nil {
		t.Fatal("expected a permission error loading an unreadable key")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("uid=%d", os.Getuid())) {
		t.Errorf("Build error not annotated with uid: %q", err.Error())
	}
}
