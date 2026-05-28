package config

import (
	"crypto/tls"
	"os"
	"testing"
)

// proxmoxEnvKeys lists every env var the Proxmox TLS plumbing reads. Tests
// that exercise these must clear them up-front to prevent leakage from the
// host environment or sibling tests.
var proxmoxEnvKeys = []string{
	"DNSWEAVER_PROXMOX_URL",
	"DNSWEAVER_PROXMOX_TOKEN_ID",
	"DNSWEAVER_PROXMOX_TOKEN_SECRET",
	"DNSWEAVER_PROXMOX_VERIFY_TLS",
	"DNSWEAVER_PROXMOX_TLS_CA_FILE",
	"DNSWEAVER_PROXMOX_TLS_CERT_FILE",
	"DNSWEAVER_PROXMOX_TLS_KEY_FILE",
	"DNSWEAVER_PROXMOX_TLS_SERVER_NAME",
	"DNSWEAVER_PROXMOX_TLS_SKIP_VERIFY",
	"DNSWEAVER_PROXMOX_TLS_MIN_VERSION",
}

func clearProxmoxEnv(t *testing.T) {
	t.Helper()
	for _, k := range proxmoxEnvKeys {
		_ = os.Unsetenv(k)
	}
}

func TestProxmoxTLS_NewCanonicalEnv(t *testing.T) {
	clearProxmoxEnv(t)
	defer clearProxmoxEnv(t)

	t.Setenv("DNSWEAVER_PROXMOX_TLS_CA_FILE", "/etc/ssl/internal-ca.pem")
	t.Setenv("DNSWEAVER_PROXMOX_TLS_CERT_FILE", "/etc/ssl/client.crt")
	t.Setenv("DNSWEAVER_PROXMOX_TLS_KEY_FILE", "/etc/ssl/client.key")
	t.Setenv("DNSWEAVER_PROXMOX_TLS_SERVER_NAME", "pve.internal")
	t.Setenv("DNSWEAVER_PROXMOX_TLS_SKIP_VERIFY", "false")
	t.Setenv("DNSWEAVER_PROXMOX_TLS_MIN_VERSION", "1.3")

	cfg := &Config{Global: &GlobalConfig{
		ProxmoxTLSCAFile:     os.Getenv("DNSWEAVER_PROXMOX_TLS_CA_FILE"),
		ProxmoxTLSCertFile:   os.Getenv("DNSWEAVER_PROXMOX_TLS_CERT_FILE"),
		ProxmoxTLSKeyFile:    os.Getenv("DNSWEAVER_PROXMOX_TLS_KEY_FILE"),
		ProxmoxTLSServerName: os.Getenv("DNSWEAVER_PROXMOX_TLS_SERVER_NAME"),
		ProxmoxTLSMinVersion: os.Getenv("DNSWEAVER_PROXMOX_TLS_MIN_VERSION"),
	}}

	got := cfg.ProxmoxTLS()
	if got == nil {
		t.Fatal("ProxmoxTLS() returned nil despite populated config")
	}
	if got.CAFile != "/etc/ssl/internal-ca.pem" {
		t.Errorf("CAFile = %q, want /etc/ssl/internal-ca.pem", got.CAFile)
	}
	if got.CertFile != "/etc/ssl/client.crt" || got.KeyFile != "/etc/ssl/client.key" {
		t.Errorf("client keypair lost: cert=%q key=%q", got.CertFile, got.KeyFile)
	}
	if got.ServerName != "pve.internal" {
		t.Errorf("ServerName = %q", got.ServerName)
	}
	if got.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = %#x, want %#x", got.MinVersion, tls.VersionTLS13)
	}
}

func TestProxmoxTLS_EmptyReturnsNil(t *testing.T) {
	clearProxmoxEnv(t)
	defer clearProxmoxEnv(t)

	cfg := &Config{Global: &GlobalConfig{}}
	if got := cfg.ProxmoxTLS(); got != nil {
		t.Errorf("expected nil for empty config, got %+v", got)
	}
}

func TestProxmoxTLS_InvalidMinVersionIgnored(t *testing.T) {
	clearProxmoxEnv(t)
	defer clearProxmoxEnv(t)

	cfg := &Config{Global: &GlobalConfig{
		ProxmoxTLSCAFile:     "/x",
		ProxmoxTLSMinVersion: "1.1", // invalid: TLS 1.1 floor not supported
	}}
	got := cfg.ProxmoxTLS()
	if got == nil {
		t.Fatal("expected non-nil since CAFile is set")
	}
	if got.MinVersion != 0 {
		t.Errorf("invalid MinVersion should fall through to default (0), got %#x", got.MinVersion)
	}
}

func TestProxmoxTLS_LegacyVerifyTLSMigration(t *testing.T) {
	// VERIFY_TLS=false (legacy default = self-signed) → InsecureSkip=true
	clearProxmoxEnv(t)
	t.Setenv("DNSWEAVER_PROXMOX_URL", "https://pve.example:8006")
	t.Setenv("DNSWEAVER_PROXMOX_TOKEN_ID", "u@pam!t")
	t.Setenv("DNSWEAVER_PROXMOX_TOKEN_SECRET", "secret")
	t.Setenv("DNSWEAVER_PROXMOX_VERIFY_TLS", "false")

	g, errs := loadGlobalConfig()
	if len(errs) > 0 {
		t.Fatalf("loadGlobalConfig errs: %v", errs)
	}
	cfg := &Config{Global: g}

	// The deprecated accessor must still reflect the operator's intent.
	if cfg.ProxmoxVerifyTLS() {
		t.Error("ProxmoxVerifyTLS() should be false (legacy passthrough)")
	}
	tlsCfg := cfg.ProxmoxTLS()
	if tlsCfg == nil || !tlsCfg.InsecureSkip {
		t.Errorf("legacy VERIFY_TLS=false should migrate to TLSConfig{InsecureSkip: true}, got %+v", tlsCfg)
	}
}

func TestProxmoxTLS_LegacyVerifyTLSTrueLeavesTLSNil(t *testing.T) {
	// VERIFY_TLS=true (operator already wanted verification) → no TLSConfig needed.
	clearProxmoxEnv(t)
	t.Setenv("DNSWEAVER_PROXMOX_URL", "https://pve.example:8006")
	t.Setenv("DNSWEAVER_PROXMOX_TOKEN_ID", "u@pam!t")
	t.Setenv("DNSWEAVER_PROXMOX_TOKEN_SECRET", "secret")
	t.Setenv("DNSWEAVER_PROXMOX_VERIFY_TLS", "true")

	g, errs := loadGlobalConfig()
	if len(errs) > 0 {
		t.Fatalf("loadGlobalConfig errs: %v", errs)
	}
	cfg := &Config{Global: g}

	if !cfg.ProxmoxVerifyTLS() {
		t.Error("ProxmoxVerifyTLS() should be true")
	}
	if tlsCfg := cfg.ProxmoxTLS(); tlsCfg != nil {
		t.Errorf("legacy VERIFY_TLS=true should not synthesize a TLSConfig, got %+v", tlsCfg)
	}
}

func TestProxmoxTLS_NewWinsOverLegacy(t *testing.T) {
	// When both new TLS_SKIP_VERIFY and legacy VERIFY_TLS are set the new
	// canonical value must win.
	clearProxmoxEnv(t)
	t.Setenv("DNSWEAVER_PROXMOX_URL", "https://pve.example:8006")
	t.Setenv("DNSWEAVER_PROXMOX_TOKEN_ID", "u@pam!t")
	t.Setenv("DNSWEAVER_PROXMOX_TOKEN_SECRET", "secret")
	t.Setenv("DNSWEAVER_PROXMOX_TLS_SKIP_VERIFY", "true")
	t.Setenv("DNSWEAVER_PROXMOX_VERIFY_TLS", "true") // conflicting legacy hint

	g, errs := loadGlobalConfig()
	if len(errs) > 0 {
		t.Fatalf("loadGlobalConfig errs: %v", errs)
	}
	cfg := &Config{Global: g}

	tlsCfg := cfg.ProxmoxTLS()
	if tlsCfg == nil || !tlsCfg.InsecureSkip {
		t.Errorf("new TLS_SKIP_VERIFY=true must win, got %+v", tlsCfg)
	}
}
