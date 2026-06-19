package powerdns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/httputil"
	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// apiRecord is a single record entry within a PowerDNS rrset.
type apiRecord struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

// rrset is a PowerDNS resource record set: all records sharing a name+type.
// ChangeType is only set on PATCH requests ("REPLACE" or "DELETE").
type rrset struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	TTL        int         `json:"ttl,omitempty"`
	ChangeType string      `json:"changetype,omitempty"`
	Records    []apiRecord `json:"records"`
}

// zoneResponse is the subset of the PowerDNS zone object dnsweaver consumes.
type zoneResponse struct {
	Name   string  `json:"name"`
	RRsets []rrset `json:"rrsets"`
}

// patchRequest is the body of a PATCH .../zones/{zone} call.
type patchRequest struct {
	RRsets []rrset `json:"rrsets"`
}

// apiErrorBody is the PowerDNS error envelope ({"error": "..."}).
type apiErrorBody struct {
	Error string `json:"error"`
}

// canonicalize ensures a name ends in exactly one trailing dot, the canonical
// form PowerDNS uses for rrset names and CNAME/SRV targets.
func canonicalize(name string) string {
	if name == "" {
		return "."
	}
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

// stripDot removes a single trailing dot so the rest of dnsweaver sees bare names.
func stripDot(name string) string {
	return strings.TrimSuffix(name, ".")
}

// quoteTXT wraps TXT content in double quotes (PowerDNS's stored form),
// escaping embedded quotes and backslashes. Callers pass dnsweaver's bare,
// unquoted value, so quoting is unconditional; unquoteTXT is its exact inverse.
func quoteTXT(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// unquoteTXT reverses quoteTXT. Unquoted input is returned unchanged.
func unquoteTXT(s string) string {
	if len(s) < 2 || !strings.HasPrefix(s, `"`) || !strings.HasSuffix(s, `"`) {
		return s
	}
	inner := s[1 : len(s)-1]
	var b strings.Builder
	escaped := false
	for _, r := range inner {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// encodeSRVContent builds PowerDNS SRV content: "<prio> <weight> <port> <target.>".
func encodeSRVContent(srv *provider.SRVData, target string) string {
	return fmt.Sprintf("%d %d %d %s", srv.Priority, srv.Weight, srv.Port, canonicalize(target))
}

// parseSRVContent parses PowerDNS SRV content into SRVData and the bare target.
func parseSRVContent(content string) (*provider.SRVData, string, error) {
	fields := strings.Fields(content)
	if len(fields) != 4 {
		return nil, "", fmt.Errorf("invalid SRV content %q: expected 4 fields", content)
	}
	prio, err := strconv.ParseUint(fields[0], 10, 16)
	if err != nil {
		return nil, "", fmt.Errorf("invalid SRV priority %q: %w", fields[0], err)
	}
	weight, err := strconv.ParseUint(fields[1], 10, 16)
	if err != nil {
		return nil, "", fmt.Errorf("invalid SRV weight %q: %w", fields[1], err)
	}
	port, err := strconv.ParseUint(fields[2], 10, 16)
	if err != nil {
		return nil, "", fmt.Errorf("invalid SRV port %q: %w", fields[2], err)
	}
	return &provider.SRVData{
		Priority: uint16(prio),
		Weight:   uint16(weight),
		Port:     uint16(port),
	}, stripDot(fields[3]), nil
}

// recordContent converts a provider.Record's value into PowerDNS rrset content.
func recordContent(rec provider.Record) (string, error) {
	switch rec.Type {
	case provider.RecordTypeA, provider.RecordTypeAAAA:
		return rec.Target, nil
	case provider.RecordTypeCNAME:
		return canonicalize(rec.Target), nil
	case provider.RecordTypeTXT:
		return quoteTXT(rec.Target), nil
	case provider.RecordTypeSRV:
		if rec.SRV == nil {
			return "", fmt.Errorf("SRV record requires SRV data")
		}
		return encodeSRVContent(rec.SRV, rec.Target), nil
	default:
		return "", fmt.Errorf("unsupported record type %v", rec.Type)
	}
}

// decodeContent converts PowerDNS rrset content back into provider.Record fields.
func decodeContent(rt provider.RecordType, content string) (target string, srv *provider.SRVData, err error) {
	switch rt {
	case provider.RecordTypeA, provider.RecordTypeAAAA:
		return content, nil, nil
	case provider.RecordTypeCNAME:
		return stripDot(content), nil, nil
	case provider.RecordTypeTXT:
		return unquoteTXT(content), nil, nil
	case provider.RecordTypeSRV:
		s, tgt, perr := parseSRVContent(content)
		if perr != nil {
			return "", nil, perr
		}
		return tgt, s, nil
	default:
		return "", nil, fmt.Errorf("unsupported record type %v", rt)
	}
}

// apiVersion is the PowerDNS HTTP API version segment.
const apiVersion = "v1"

// errZoneNotFound is returned when the PowerDNS API responds 404 for the zone.
// The provider translates it into an actionable, operator-facing message.
var errZoneNotFound = errors.New("zone not found")

// Client is a PowerDNS Authoritative HTTP API client.
type Client struct {
	baseURL    string // e.g. http://ns1:8081 (no trailing slash, no /api/v1)
	apiKey     string
	serverID   string
	httpClient *http.Client
	logger     *slog.Logger
}

// ClientOption is a functional option for configuring the Client.
type ClientOption func(*Client)

// WithHTTPClient sets a pre-configured HTTP client (with shared TLS/timeout/UA).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) ClientOption {
	return func(c *Client) {
		if l != nil {
			c.logger = l
		}
	}
}

// NewClient creates a PowerDNS API client. baseURL is the server root
// (e.g. http://ns1:8081); /api/v1/... paths are appended internally.
func NewClient(baseURL, apiKey, serverID string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		serverID:   serverID,
		httpClient: http.DefaultClient,
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// zonePath builds the API path for the configured zone.
func (c *Client) zonePath(zone string) string {
	return fmt.Sprintf("/api/%s/servers/%s/zones/%s",
		apiVersion, url.PathEscape(c.serverID), url.PathEscape(canonicalize(zone)))
}

// doRequest performs an HTTP request and returns the 2xx body. Non-2xx and
// transport errors are mapped to package/shared sentinels via mapError.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.logger.Debug("powerdns API request",
		slog.String("method", method),
		slog.String("path", path),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", provider.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	respBody, err := httputil.ReadBody(resp, 0)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, c.mapError(resp.StatusCode, respBody)
	}
	return respBody, nil
}

// mapError translates a non-2xx response into an error wrapping the
// appropriate sentinel. PowerDNS returns {"error": "..."} bodies.
func (c *Client) mapError(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	var e apiErrorBody
	if json.Unmarshal(body, &e) == nil && e.Error != "" {
		msg = e.Error
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Errorf("%w: %s", provider.ErrUnauthorized, msg)
	case status == http.StatusNotFound:
		return errZoneNotFound
	case status == http.StatusUnprocessableEntity:
		return fmt.Errorf("powerdns rejected request (422): %s", msg)
	case status >= 500:
		return fmt.Errorf("%w: server returned %d: %s", provider.ErrProviderUnavailable, status, msg)
	default:
		return fmt.Errorf("powerdns API error (status %d): %s", status, msg)
	}
}

// Ping verifies API connectivity and authentication by listing servers. It
// does not check for a specific zone (that is the provider's responsibility).
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/%s/servers", apiVersion), nil)
	return err
}

// GetZone fetches the full zone object including its rrsets.
func (c *Client) GetZone(ctx context.Context, zone string) (*zoneResponse, error) {
	body, err := c.doRequest(ctx, http.MethodGet, c.zonePath(zone), nil)
	if err != nil {
		return nil, err
	}
	var z zoneResponse
	if err := json.Unmarshal(body, &z); err != nil {
		return nil, fmt.Errorf("parsing zone response: %w", err)
	}
	return &z, nil
}

// PatchRRsets applies one or more rrset changes (REPLACE/DELETE) to the zone.
func (c *Client) PatchRRsets(ctx context.Context, zone string, rrsets []rrset) error {
	payload, err := json.Marshal(patchRequest{RRsets: rrsets})
	if err != nil {
		return fmt.Errorf("marshaling patch request: %w", err)
	}
	_, err = c.doRequest(ctx, http.MethodPatch, c.zonePath(zone), bytes.NewReader(payload))
	return err
}
