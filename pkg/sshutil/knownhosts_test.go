package sshutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// newTestHostKey generates an ed25519 SSH public key for use in tests.
func newTestHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating ed25519 key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("converting to ssh public key: %v", err)
	}
	return sshPub
}

// writeKnownHosts writes a known_hosts file containing the given host/key pairs
// and returns its path.
func writeKnownHosts(t *testing.T, entries map[string]ssh.PublicKey) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")

	var content []byte
	for host, key := range entries {
		content = append(content, []byte(knownhosts.Line([]string{host}, key))...)
		content = append(content, '\n')
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("writing known_hosts: %v", err)
	}
	return path
}

func TestBuildHostKeyCallback_KnownHostsMatch(t *testing.T) {
	host := "127.0.0.1:22"
	key := newTestHostKey(t)
	path := writeKnownHosts(t, map[string]ssh.PublicKey{host: key})

	c := &Client{
		config: &Config{
			Host:            "127.0.0.1",
			Port:            22,
			User:            "test",
			Password:        "test",
			HostKeyCallback: path,
		},
		logger: slog.Default(),
	}

	cb, err := c.buildHostKeyCallback()
	if err != nil {
		t.Fatalf("buildHostKeyCallback() error = %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}
	if err := cb(host, addr, key); err != nil {
		t.Errorf("expected matching host key to verify, got error: %v", err)
	}
}

func TestBuildHostKeyCallback_KnownHostsMismatch(t *testing.T) {
	host := "127.0.0.1:22"
	trusted := newTestHostKey(t)
	attacker := newTestHostKey(t)
	path := writeKnownHosts(t, map[string]ssh.PublicKey{host: trusted})

	c := &Client{
		config: &Config{
			Host:            "127.0.0.1",
			Port:            22,
			User:            "test",
			Password:        "test",
			HostKeyCallback: path,
		},
		logger: slog.Default(),
	}

	cb, err := c.buildHostKeyCallback()
	if err != nil {
		t.Fatalf("buildHostKeyCallback() error = %v", err)
	}

	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}
	if err := cb(host, addr, attacker); err == nil {
		t.Error("expected mismatched host key to be rejected, got nil error")
	}
}

func TestBuildHostKeyCallback_UnknownHost(t *testing.T) {
	known := newTestHostKey(t)
	path := writeKnownHosts(t, map[string]ssh.PublicKey{"10.0.0.1:22": known})

	c := &Client{
		config: &Config{
			Host:            "127.0.0.1",
			Port:            22,
			User:            "test",
			Password:        "test",
			HostKeyCallback: path,
		},
		logger: slog.Default(),
	}

	cb, err := c.buildHostKeyCallback()
	if err != nil {
		t.Fatalf("buildHostKeyCallback() error = %v", err)
	}

	// A host not present in known_hosts must be rejected.
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}
	if err := cb("127.0.0.1:22", addr, newTestHostKey(t)); err == nil {
		t.Error("expected unknown host to be rejected, got nil error")
	}
}

func TestBuildHostKeyCallback_StrictWithoutFile(t *testing.T) {
	c := &Client{
		config: &Config{
			Host:                  "127.0.0.1",
			Port:                  22,
			User:                  "test",
			Password:              "test",
			StrictHostKeyChecking: true,
		},
		logger: slog.Default(),
	}

	if _, err := c.buildHostKeyCallback(); err == nil {
		t.Error("expected error when strict host key checking is enabled without a known_hosts file")
	}
}

func TestBuildHostKeyCallback_StrictWithIgnoreConflict(t *testing.T) {
	c := &Client{
		config: &Config{
			Host:                  "127.0.0.1",
			Port:                  22,
			User:                  "test",
			Password:              "test",
			HostKeyCallback:       "ignore",
			StrictHostKeyChecking: true,
		},
		logger: slog.Default(),
	}

	if _, err := c.buildHostKeyCallback(); err == nil {
		t.Error("expected error when strict checking is combined with HOST_KEY_CALLBACK=ignore")
	}
}

func TestBuildHostKeyCallback_MissingKnownHostsFile(t *testing.T) {
	c := &Client{
		config: &Config{
			Host:            "127.0.0.1",
			Port:            22,
			User:            "test",
			Password:        "test",
			HostKeyCallback: filepath.Join(t.TempDir(), "does-not-exist"),
		},
		logger: slog.Default(),
	}

	if _, err := c.buildHostKeyCallback(); err == nil {
		t.Error("expected error when known_hosts file does not exist")
	}
}

func TestBuildHostKeyCallback_InsecureDefault(t *testing.T) {
	c := &Client{
		config: &Config{
			Host:     "127.0.0.1",
			Port:     22,
			User:     "test",
			Password: "test",
		},
		logger: slog.Default(),
	}

	cb, err := c.buildHostKeyCallback()
	if err != nil {
		t.Fatalf("buildHostKeyCallback() error = %v", err)
	}
	if cb == nil {
		t.Fatal("expected an insecure-ignore callback, got nil")
	}
	// Insecure-ignore accepts any key.
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}
	if err := cb("127.0.0.1:22", addr, newTestHostKey(t)); err != nil {
		t.Errorf("insecure-ignore callback should accept any key, got: %v", err)
	}
}
