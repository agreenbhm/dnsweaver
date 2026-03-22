// Package httputil provides shared HTTP client utilities for DNSWeaver providers.
package httputil

import (
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// Default HTTP client configuration values.
const (
	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second

	// DefaultUserAgent is used when no custom user agent is specified.
	DefaultUserAgent = "dnsweaver/1.0"
)

// ClientConfig contains configuration for creating an HTTP client.
type ClientConfig struct {
	// Timeout is the HTTP client timeout. Defaults to 30 seconds.
	Timeout time.Duration

	// TLSSkipVerify controls whether to skip TLS certificate verification.
	// WARNING: This should only be used for testing or when connecting to
	// servers with self-signed certificates. It is insecure for production.
	TLSSkipVerify bool

	// UserAgent is the User-Agent header to set on requests.
	// Defaults to "dnsweaver/1.0" if not specified.
	UserAgent string

	// Logger enables debug logging for HTTP requests.
	// If nil, no debug logging is performed.
	Logger *slog.Logger
}

// userAgentTransport wraps an http.RoundTripper to add User-Agent header
// and optionally log requests at debug level.
type userAgentTransport struct {
	base      http.RoundTripper
	userAgent string
	logger    *slog.Logger
}

// sensitiveQueryParams lists query parameter names that may contain credentials.
// These are redacted from URLs before logging to prevent secret leakage.
var sensitiveQueryParams = map[string]bool{
	"token":    true,
	"auth":     true,
	"password": true,
	"key":      true,
	"secret":   true,
	"sid":      true,
	"apikey":   true,
}

// sanitizeURL returns a URL string with sensitive query parameters redacted.
// This prevents credentials from appearing in debug logs.
func sanitizeURL(u *url.URL) string {
	if u == nil {
		return ""
	}

	query := u.Query()
	redacted := false
	for param := range query {
		if sensitiveQueryParams[param] {
			query.Set(param, "REDACTED")
			redacted = true
		}
	}

	if !redacted {
		return u.String()
	}

	// Rebuild URL with redacted query
	sanitized := *u
	sanitized.RawQuery = query.Encode()
	return sanitized.String()
}

// RoundTrip implements http.RoundTripper.
func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Set User-Agent if not already set
	if req.Header.Get("User-Agent") == "" && t.userAgent != "" {
		req.Header.Set("User-Agent", t.userAgent)
	}

	// Debug log the request (with sensitive query params redacted)
	if t.logger != nil {
		t.logger.Debug("HTTP request",
			slog.String("method", req.Method),
			slog.String("url", sanitizeURL(req.URL)),
		)
	}

	resp, err := t.base.RoundTrip(req)

	// Debug log the response (with sensitive query params redacted)
	if t.logger != nil && resp != nil {
		t.logger.Debug("HTTP response",
			slog.String("method", req.Method),
			slog.String("url", sanitizeURL(req.URL)),
			slog.Int("status", resp.StatusCode),
		)
	}

	return resp, err
}

// NewClient creates an HTTP client with the specified configuration.
// If cfg is nil, defaults are used (30s timeout, TLS verification enabled).
func NewClient(cfg *ClientConfig) *http.Client {
	if cfg == nil {
		cfg = &ClientConfig{}
	}

	// Apply defaults
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	userAgent := cfg.UserAgent
	if userAgent == "" {
		userAgent = DefaultUserAgent
	}

	// Start with default transport
	baseTransport := http.DefaultTransport

	// Configure TLS if needed
	if cfg.TLSSkipVerify {
		baseTransport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec // Intentional: user explicitly requested skip
			},
		}
	}

	// Wrap with User-Agent and logging transport
	transport := &userAgentTransport{
		base:      baseTransport,
		userAgent: userAgent,
		logger:    cfg.Logger,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// NewClientWithTransport creates an HTTP client with custom transport settings.
// This allows advanced configuration like custom TLS roots, proxies, etc.
func NewClientWithTransport(timeout time.Duration, transport *http.Transport) *http.Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// DefaultClient returns a new HTTP client with default settings.
// Equivalent to NewClient(nil).
func DefaultClient() *http.Client {
	return NewClient(nil)
}

// Response size limits for protection against OOM from oversized responses.
const (
	// MaxResponseSize is the default maximum response body size (10 MB).
	// This prevents OOM if a server sends an unexpectedly large response.
	MaxResponseSize int64 = 10 * 1024 * 1024
)

// ReadBody reads and returns the response body, limiting the size to prevent OOM.
// If maxBytes is 0, MaxResponseSize is used.
// Returns an error if the response exceeds the limit.
func ReadBody(resp *http.Response, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = MaxResponseSize
	}

	// Read up to maxBytes+1 to detect if the response exceeds the limit
	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds maximum size of %d bytes", maxBytes)
	}

	return data, nil
}
