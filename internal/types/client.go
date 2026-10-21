package types

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Service identifies one of the SAP communication management OData services.
// Each value is the service root path relative to the host.
type Service string

const (
	ServiceCommunicationSystem      Service = "/sap/opu/odata4/sap/aps_com_cs_a4c_odata/srvd_a2x/sap/aps_com_cs_a4c_odata/0001"
	ServiceCommunicationUser        Service = "/sap/opu/odata4/sap/aps_com_cu_a4c_odata/srvd_a2x/sap/aps_com_cu_a4c_odata/0001"
	ServiceCommunicationArrangement Service = "/sap/opu/odata4/sap/aps_com_ca_a4c_odata/srvd_a2x/sap/aps_com_ca_a4c_odata/0001"
)

// sensitiveFields are request body keys that are never written to the logs.
var sensitiveFields = map[string]bool{"Password": true}

// ClientConfig contains the configuration for the SAP S/4HANA API client.
// Set either Username and Password (basic authentication) or
// ClientCertificate and ClientKey (X.509 client certificate, mTLS).
type ClientConfig struct {
	// Host is the API host name, reached over HTTPS on the default port. A
	// full URL (e.g. "http://127.0.0.1:8080") is also accepted, which the
	// tests use to reach a local mock server.
	Host     string
	Username string
	Password string

	// ClientCertificate and ClientKey are PEM encoded. The certificate may
	// be followed by its intermediate certificates.
	ClientCertificate string
	ClientKey         string

	// RootCAs verifies the server certificate; nil uses the system roots.
	RootCAs *x509.CertPool
}

// Client represents the SAP S/4HANA API client
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Username   string
	Password   string
	// mu serializes modifying requests: SAP binds the CSRF token to the session
	// cookie, and concurrent token fetches for different services would replace
	// each other's session in the shared cookie jar.
	mu sync.Mutex
}

// NewClient creates a new SAP S/4HANA API client
func NewClient(config *ClientConfig) (*Client, error) {
	// Create a cookie jar to maintain session cookies (required for CSRF tokens)
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	baseURL := strings.TrimSuffix(config.Host, "/")
	if !strings.Contains(baseURL, "://") {
		baseURL = "https://" + baseURL
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: config.RootCAs}
	if config.ClientCertificate != "" || config.ClientKey != "" {
		cert, err := tls.X509KeyPair([]byte(config.ClientCertificate), []byte(config.ClientKey))
		if err != nil {
			return nil, fmt.Errorf("invalid client certificate or key: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout:   time.Second * 30,
			Jar:       jar,
			Transport: transport,
		},
		Username: config.Username,
		Password: config.Password,
	}, nil
}

// authenticate adds basic authentication unless a client certificate is used.
func (c *Client) authenticate(req *http.Request) {
	if c.Username != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
}

// ReadPEM returns value if it contains PEM data, and otherwise reads the
// file it names. Environment variables accept either form.
func ReadPEM(value string) (string, error) {
	if value == "" || strings.Contains(value, "-----BEGIN") {
		return value, nil
	}
	content, err := os.ReadFile(value)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", value, err)
	}
	return string(content), nil
}

// serviceBaseURL returns the base URL for a specific service
func (c *Client) serviceBaseURL(service Service) string {
	return c.BaseURL + string(service)
}

// fetchCSRFToken fetches a CSRF token for the given service
func (c *Client) fetchCSRFToken(ctx context.Context, service Service) (string, error) {
	// The service root without a trailing slash answers with a redirect
	baseURL := c.serviceBaseURL(service) + "/"

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, baseURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create CSRF request: %w", err)
	}

	c.authenticate(req)
	req.Header.Set("X-CSRF-Token", "Fetch")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch CSRF token: %w", err)
	}
	defer resp.Body.Close()

	tflog.Debug(ctx, "SAP CSRF token fetch", map[string]interface{}{"url": baseURL, "status": resp.StatusCode})

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", authenticationError(resp.StatusCode)
	}

	csrfToken := resp.Header.Get("X-CSRF-Token")
	if csrfToken == "" {
		return "", fmt.Errorf("no CSRF token found in response headers (status %d)", resp.StatusCode)
	}
	return csrfToken, nil
}

// makeRequest performs an HTTP request to the SAP S/4HANA API. endpoint is
// relative to the service root and may include a query string.
func (c *Client) makeRequest(ctx context.Context, method string, service Service, endpoint string, body interface{}) (*http.Response, error) {
	var buf io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to encode request body: %w", err)
		}
		buf = bytes.NewReader(encoded)
	}

	path, query, _ := strings.Cut(endpoint, "?")
	reqURL, err := url.JoinPath(c.serviceBaseURL(service), path)
	if err != nil {
		return nil, fmt.Errorf("failed to build request URL: %w", err)
	}
	if query != "" {
		reqURL += "?" + query
	}

	logFields := map[string]interface{}{"method": method, "url": reqURL}
	if body != nil {
		logFields["body"] = redact(body)
	}
	tflog.Debug(ctx, "SAP API request", logFields)

	req, err := http.NewRequestWithContext(ctx, method, reqURL, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.authenticate(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Modifying requests need a CSRF token
	if method != http.MethodGet {
		c.mu.Lock()
		defer c.mu.Unlock()
		csrfToken, err := c.fetchCSRFToken(ctx, service)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-CSRF-Token", csrfToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}

	tflog.Debug(ctx, "SAP API response", map[string]interface{}{"method": method, "url": reqURL, "status": resp.StatusCode})
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		return nil, authenticationError(resp.StatusCode)
	}
	return resp, nil
}

func authenticationError(status int) error {
	return fmt.Errorf("authentication failed (status %d): check the communication user's password or client certificate, "+
		"and that the user is assigned to a communication arrangement for the communication management APIs", status)
}

// redact returns a copy of a request body suitable for logging.
func redact(body interface{}) interface{} {
	m, ok := body.(map[string]interface{})
	if !ok {
		return body
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if sensitiveFields[k] {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, service Service, endpoint string) (*http.Response, error) {
	return c.makeRequest(ctx, http.MethodGet, service, endpoint, nil)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, service Service, endpoint string, body interface{}) (*http.Response, error) {
	return c.makeRequest(ctx, http.MethodPost, service, endpoint, body)
}

// Patch performs a PATCH request
func (c *Client) Patch(ctx context.Context, service Service, endpoint string, body interface{}) (*http.Response, error) {
	return c.makeRequest(ctx, http.MethodPatch, service, endpoint, body)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, service Service, endpoint string) (*http.Response, error) {
	return c.makeRequest(ctx, http.MethodDelete, service, endpoint, nil)
}

// ReadResponseBody reads and returns the response body as a byte slice
func ReadResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return body, nil
}

// IsErrorResponse checks if the HTTP response indicates an error
func IsErrorResponse(resp *http.Response) bool {
	return resp.StatusCode >= 400
}
