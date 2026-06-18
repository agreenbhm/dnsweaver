// Package ovh implements the dnsweaver provider interface for OVHcloud DNS.
//
// OVH exposes DNS zone management through its public API. Each request is
// authenticated with an application key, application secret, and consumer key,
// and signed with the OVH server time. Zone modifications are staged and only
// take effect after an explicit zone refresh, which this client issues after
// every mutating operation.
package ovh

import (
	"context"
	"crypto/sha1" //nolint:gosec // OVH API mandates SHA-1 request signatures
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/httputil"
)

// ovhRecord represents a DNS record as returned by the OVH API.
type ovhRecord struct {
	ID        int64  `json:"id"`
	Zone      string `json:"zone"`
	SubDomain string `json:"subDomain"`
	FieldType string `json:"fieldType"`
	Target    string `json:"target"`
	TTL       int    `json:"ttl"`
}

// recordCreateRequest is the body for creating a DNS record.
type recordCreateRequest struct {
	FieldType string `json:"fieldType"`
	SubDomain string `json:"subDomain"`
	Target    string `json:"target"`
	TTL       int    `json:"ttl,omitempty"`
}

// recordUpdateRequest is the body for updating a DNS record.
// OVH does not allow changing fieldType on update, so it is omitted.
type recordUpdateRequest struct {
	SubDomain string `json:"subDomain"`
	Target    string `json:"target"`
	TTL       int    `json:"ttl,omitempty"`
}

// apiError represents an OVH API error response body.
type apiError struct {
	Message   string `json:"message"`
	ErrorCode string `json:"errorCode"`
	HTTPCode  string `json:"httpCode"`
}

// Client is an OVHcloud DNS API client.
type Client struct {
	baseURL     string
	appKey      string
	appSecret   string
	consumerKey string
	httpClient  *http.Client
	logger      *slog.Logger

	// timeDeltaOnce ensures the server-time offset is fetched only once.
	timeDeltaOnce sync.Once
	timeDelta     int64
	timeDeltaErr  error
}

// ClientOption is a functional option for configuring the Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
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

// NewClient creates a new OVH API client for the given endpoint base URL.
func NewClient(baseURL, appKey, appSecret, consumerKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		appKey:      appKey,
		appSecret:   appSecret,
		consumerKey: consumerKey,
		httpClient:  httputil.DefaultClient(),
		logger:      slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// nowFunc returns the current Unix time. Overridable in tests.
var nowFunc = func() int64 { return time.Now().Unix() }

// timeDelta returns the offset between OVH server time and local time.
// OVH signatures embed a timestamp that must be close to the server's clock,
// so the offset is fetched once from /auth/time and reused.
func (c *Client) getTimeDelta(ctx context.Context) (int64, error) {
	c.timeDeltaOnce.Do(func() {
		reqURL := c.baseURL + "/auth/time"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			c.timeDeltaErr = fmt.Errorf("creating time request: %w", err)
			return
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.timeDeltaErr = fmt.Errorf("fetching server time: %w", err)
			return
		}
		defer resp.Body.Close()

		body, err := httputil.ReadBody(resp, 0)
		if err != nil {
			c.timeDeltaErr = fmt.Errorf("reading server time: %w", err)
			return
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			c.timeDeltaErr = fmt.Errorf("fetching server time: unexpected status %d: %s", resp.StatusCode, string(body))
			return
		}

		serverTime, err := strconv.ParseInt(strings.TrimSpace(string(body)), 10, 64)
		if err != nil {
			c.timeDeltaErr = fmt.Errorf("parsing server time %q: %w", string(body), err)
			return
		}

		c.timeDelta = serverTime - nowFunc()
	})

	return c.timeDelta, c.timeDeltaErr
}

// sign computes the OVH request signature for the given method, full URL, and body.
// Format: "$1$" + SHA1_HEX(appSecret+"+"+consumerKey+"+"+method+"+"+url+"+"+body+"+"+timestamp)
func (c *Client) sign(method, fullURL, body string, timestamp int64) string {
	h := sha1.New() //nolint:gosec // OVH API mandates SHA-1 request signatures
	// writing to a hash.Hash never errors; assign to blanks to satisfy errcheck
	_, _ = fmt.Fprintf(h, "%s+%s+%s+%s+%s+%d",
		c.appSecret, c.consumerKey, method, fullURL, body, timestamp)
	return "$1$" + fmt.Sprintf("%x", h.Sum(nil))
}

// doRequest performs a signed request against the OVH API and returns the
// raw response body for 2xx responses.
func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	delta, err := c.getTimeDelta(ctx)
	if err != nil {
		return nil, err
	}

	fullURL := c.baseURL + path
	timestamp := nowFunc() + delta

	var bodyReader io.Reader
	bodyStr := ""
	if len(body) > 0 {
		bodyStr = string(body)
		bodyReader = strings.NewReader(bodyStr)
	}

	c.logger.Debug("making API request",
		slog.String("method", method),
		slog.String("path", path),
	)

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-Ovh-Application", c.appKey)
	req.Header.Set("X-Ovh-Consumer", c.consumerKey)
	req.Header.Set("X-Ovh-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Ovh-Signature", c.sign(method, fullURL, bodyStr, timestamp))
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := httputil.ReadBody(resp, 0)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		if jsonErr := json.Unmarshal(respBody, &apiErr); jsonErr == nil && apiErr.Message != "" {
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiErr.Message)
		}
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Ping verifies connectivity and credentials by fetching the configured zone's
// SOA record. This confirms both that the API is reachable and that the
// credentials are authorized for the zone.
func (c *Client) Ping(ctx context.Context, zone string) error {
	path := fmt.Sprintf("/domain/zone/%s/soa", url.PathEscape(zone))
	if _, err := c.doRequest(ctx, http.MethodGet, path, nil); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}

// ListRecordIDs returns the record IDs in the zone, optionally filtered by
// field type and subdomain. Empty filters return all records.
func (c *Client) ListRecordIDs(ctx context.Context, zone, fieldType, subDomain string) ([]int64, error) {
	params := url.Values{}
	if fieldType != "" {
		params.Set("fieldType", fieldType)
	}
	if subDomain != "" {
		params.Set("subDomain", subDomain)
	}

	path := fmt.Sprintf("/domain/zone/%s/record", url.PathEscape(zone))
	if q := params.Encode(); q != "" {
		path += "?" + q
	}

	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("listing record IDs: %w", err)
	}

	var ids []int64
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, fmt.Errorf("parsing record IDs: %w", err)
	}

	return ids, nil
}

// GetRecord fetches the full details of a single record by ID.
func (c *Client) GetRecord(ctx context.Context, zone string, id int64) (*ovhRecord, error) {
	path := fmt.Sprintf("/domain/zone/%s/record/%d", url.PathEscape(zone), id)
	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("getting record %d: %w", id, err)
	}

	var rec ovhRecord
	if err := json.Unmarshal(body, &rec); err != nil {
		return nil, fmt.Errorf("parsing record %d: %w", id, err)
	}

	return &rec, nil
}

// CreateRecord creates a DNS record in the zone and returns the created record.
// The caller is responsible for issuing RefreshZone to apply the change.
func (c *Client) CreateRecord(ctx context.Context, zone, fieldType, subDomain, target string, ttl int) (*ovhRecord, error) {
	reqBody := recordCreateRequest{
		FieldType: fieldType,
		SubDomain: subDomain,
		Target:    target,
		TTL:       ttl,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	path := fmt.Sprintf("/domain/zone/%s/record", url.PathEscape(zone))
	respBody, err := c.doRequest(ctx, http.MethodPost, path, bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("creating record: %w", err)
	}

	var rec ovhRecord
	if err := json.Unmarshal(respBody, &rec); err != nil {
		return nil, fmt.Errorf("parsing created record: %w", err)
	}

	c.logger.Info("created DNS record",
		slog.String("zone", zone),
		slog.String("type", fieldType),
		slog.String("sub_domain", subDomain),
		slog.String("target", target),
		slog.Int("ttl", ttl),
	)

	return &rec, nil
}

// UpdateRecord updates an existing record by ID.
// The caller is responsible for issuing RefreshZone to apply the change.
func (c *Client) UpdateRecord(ctx context.Context, zone string, id int64, subDomain, target string, ttl int) error {
	reqBody := recordUpdateRequest{
		SubDomain: subDomain,
		Target:    target,
		TTL:       ttl,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	path := fmt.Sprintf("/domain/zone/%s/record/%d", url.PathEscape(zone), id)
	if _, err := c.doRequest(ctx, http.MethodPut, path, bodyBytes); err != nil {
		return fmt.Errorf("updating record %d: %w", id, err)
	}

	c.logger.Info("updated DNS record",
		slog.String("zone", zone),
		slog.Int64("record_id", id),
		slog.String("sub_domain", subDomain),
		slog.String("target", target),
		slog.Int("ttl", ttl),
	)

	return nil
}

// DeleteRecord deletes a record by ID.
// The caller is responsible for issuing RefreshZone to apply the change.
func (c *Client) DeleteRecord(ctx context.Context, zone string, id int64) error {
	path := fmt.Sprintf("/domain/zone/%s/record/%d", url.PathEscape(zone), id)
	if _, err := c.doRequest(ctx, http.MethodDelete, path, nil); err != nil {
		return fmt.Errorf("deleting record %d: %w", id, err)
	}

	c.logger.Info("deleted DNS record",
		slog.String("zone", zone),
		slog.Int64("record_id", id),
	)

	return nil
}

// RefreshZone applies staged zone modifications. OVH requires this call for
// created, updated, or deleted records to take effect on the DNS servers.
func (c *Client) RefreshZone(ctx context.Context, zone string) error {
	path := fmt.Sprintf("/domain/zone/%s/refresh", url.PathEscape(zone))
	if _, err := c.doRequest(ctx, http.MethodPost, path, nil); err != nil {
		return fmt.Errorf("refreshing zone: %w", err)
	}

	c.logger.Debug("refreshed zone", slog.String("zone", zone))
	return nil
}
