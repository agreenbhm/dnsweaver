package adguard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/httputil"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// rewriteEntry represents an AdGuard Home DNS rewrite rule.
type rewriteEntry struct {
	Domain  string `json:"domain"`
	Answer  string `json:"answer"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// rewriteUpdate represents a rewrite update request.
type rewriteUpdate struct {
	Target rewriteEntry `json:"target"`
	Update rewriteEntry `json:"update"`
}

// serverStatus represents the /control/status response.
type serverStatus struct {
	Version           string `json:"version"`
	Running           bool   `json:"running"`
	ProtectionEnabled bool   `json:"protection_enabled"`
}

// Client is an AdGuard Home API client.
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	logger     *slog.Logger
}

// ClientOption is a functional option for configuring the Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *Client) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// NewClient creates a new AdGuard Home API client.
func NewClient(baseURL, username, password string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		httpClient: httputil.DefaultClient(),
		logger:     slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// doRequest performs an HTTP request to the AdGuard Home API.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
	reqURL := fmt.Sprintf("%s/control%s", c.baseURL, path)

	c.logger.Debug("making AdGuard API request",
		slog.String("method", method),
		slog.String("path", path),
	)

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := httputil.ReadBody(resp, 0)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// Ping checks connectivity to AdGuard Home by calling GET /control/status.
func (c *Client) Ping(ctx context.Context) error {
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, "/status", nil)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return provider.ErrUnauthorized
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", statusCode, string(body))
	}

	var status serverStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return fmt.Errorf("parsing status response: %w", err)
	}

	if !status.Running {
		return provider.ErrProviderUnavailable
	}

	c.logger.Debug("AdGuard Home ping successful",
		slog.String("version", status.Version),
		slog.Bool("running", status.Running),
	)

	return nil
}

// List returns all DNS rewrite rules from AdGuard Home.
func (c *Client) List(ctx context.Context) ([]rewriteEntry, error) {
	body, statusCode, err := c.doRequest(ctx, http.MethodGet, "/rewrite/list", nil)
	if err != nil {
		return nil, fmt.Errorf("listing rewrites: %w", err)
	}

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return nil, provider.ErrUnauthorized
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", statusCode, string(body))
	}

	var entries []rewriteEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing rewrite list: %w", err)
	}

	return entries, nil
}

// Create adds a new DNS rewrite rule.
func (c *Client) Create(ctx context.Context, entry rewriteEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling rewrite entry: %w", err)
	}

	body, statusCode, err := c.doRequest(ctx, http.MethodPost, "/rewrite/add", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating rewrite: %w", err)
	}

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return provider.ErrUnauthorized
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", statusCode, string(body))
	}

	return nil
}

// Delete removes a DNS rewrite rule.
func (c *Client) Delete(ctx context.Context, entry rewriteEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling rewrite entry: %w", err)
	}

	body, statusCode, err := c.doRequest(ctx, http.MethodPost, "/rewrite/delete", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("deleting rewrite: %w", err)
	}

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return provider.ErrUnauthorized
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", statusCode, string(body))
	}

	return nil
}

// Update modifies an existing DNS rewrite rule.
func (c *Client) Update(ctx context.Context, target, update rewriteEntry) error {
	payload, err := json.Marshal(rewriteUpdate{
		Target: target,
		Update: update,
	})
	if err != nil {
		return fmt.Errorf("marshaling rewrite update: %w", err)
	}

	body, statusCode, err := c.doRequest(ctx, http.MethodPut, "/rewrite/update", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("updating rewrite: %w", err)
	}

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return provider.ErrUnauthorized
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", statusCode, string(body))
	}

	return nil
}
