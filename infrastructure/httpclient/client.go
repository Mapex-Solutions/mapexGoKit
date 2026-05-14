package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPClient provides a generic HTTP client for making requests to external services.
// It supports API Key authentication and automatic JSON serialization/deserialization.
//
// Features:
//   - Configurable base URL and API Key
//   - Automatic JSON marshaling/unmarshaling
//   - Context support for timeouts and cancellation
//   - Generic response type handling
//   - Customizable HTTP client (timeout, transport, etc.)
//
// Example usage:
//
//	client := httpclient.New(httpclient.Config{
//	    BaseURL: "http://localhost:5003",
//	    APIKey:  "my-secret-key",
//	    Timeout: 5 * time.Second,
//	})
//
//	var result []RouteGroupResponse
//	err := client.Get(ctx, "/api/internal/v1/routegroups?ids=id1,id2", &result)
type HTTPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	headers    map[string]string
}

// Config defines the configuration for creating a new HTTPClient.
type Config struct {
	BaseURL string        // Base URL of the service (e.g., "http://localhost:5003")
	APIKey  string        // API Key for authentication (sent as X-API-Key header)
	Timeout time.Duration // Request timeout (default: 10 seconds)
}

// New creates and returns a new HTTPClient instance.
//
// Parameters:
//   - config: Configuration for the HTTP client
//
// Returns:
//   - *HTTPClient: Configured HTTP client instance
func New(config Config) *HTTPClient {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second // Default timeout
	}

	return &HTTPClient{
		baseURL: config.BaseURL,
		apiKey:  config.APIKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		headers: make(map[string]string),
	}
}

// SetHeader registers a header that will be attached to every subsequent
// request. Passing an empty value removes the header. Use it for
// per-client identity such as Authorization, X-Org-Context, or X-Tenant
// — anything that is not part of the request body but follows the client
// for its whole lifetime.
func (c *HTTPClient) SetHeader(key, value string) {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	if value == "" {
		delete(c.headers, key)
		return
	}
	c.headers[key] = value
}

// Raw performs an HTTP request and returns the raw http.Response without
// reading the body, without unmarshaling, and without rejecting non-2xx
// status codes. Callers own the response lifetime and MUST close
// resp.Body when finished. This is the entry point used by saga journeys
// where the test asserts directly on the status code (e.g. "creation
// returns 201") rather than on a deserialized DTO.
func (c *HTTPClient) Raw(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	url := c.baseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.applyHeaders(req)

	return c.httpClient.Do(req)
}

// RawWithHeaders is identical to Raw but accepts a per-call header map
// merged on top of the headers registered with SetHeader. The override is
// scoped to this single request; it does not mutate the client. Use it for
// requests that need a header only once — refresh-token flows that send
// X-Refresh-Token, fan-in endpoints that need a specific X-Trace-Id, etc.
func (c *HTTPClient) RawWithHeaders(ctx context.Context, method, endpoint string, body any, headers map[string]string) (*http.Response, error) {
	url := c.baseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.applyHeaders(req)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.httpClient.Do(req)
}

// applyHeaders sets the standard headers (Content-Type, X-API-Key) plus
// any caller-registered headers via SetHeader. Caller headers win — they
// can override Content-Type if the body is not JSON.
func (c *HTTPClient) applyHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
}

// Get performs a GET request to the specified endpoint.
//
// The response body is automatically unmarshaled into the provided result pointer.
//
// Parameters:
//   - ctx: Context for controlling cancellation and timeouts
//   - endpoint: API endpoint path (e.g., "/api/internal/v1/routegroups?ids=id1,id2")
//   - result: Pointer to store the unmarshaled response (must be a pointer)
//
// Returns:
//   - error: If the request fails or response cannot be unmarshaled
//
// Example:
//
//	var routeGroups []RouteGroupResponse
//	err := client.Get(ctx, "/api/internal/v1/routegroups?ids=id1,id2", &routeGroups)
func (c *HTTPClient) Get(ctx context.Context, endpoint string, result interface{}) error {
	return c.doRequest(ctx, "GET", endpoint, nil, result)
}

// Post performs a POST request to the specified endpoint.
//
// The request body is automatically marshaled to JSON, and the response
// body is unmarshaled into the provided result pointer.
//
// Parameters:
//   - ctx: Context for controlling cancellation and timeouts
//   - endpoint: API endpoint path
//   - body: Request body to be marshaled to JSON
//   - result: Pointer to store the unmarshaled response (can be nil if no response expected)
//
// Returns:
//   - error: If the request fails or response cannot be unmarshaled
func (c *HTTPClient) Post(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, "POST", endpoint, body, result)
}

// Put performs a PUT request to the specified endpoint.
func (c *HTTPClient) Put(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, "PUT", endpoint, body, result)
}

// Delete performs a DELETE request to the specified endpoint.
func (c *HTTPClient) Delete(ctx context.Context, endpoint string, result interface{}) error {
	return c.doRequest(ctx, "DELETE", endpoint, nil, result)
}

// doRequest performs the actual HTTP request with proper error handling.
//
// This internal method handles:
//   - JSON marshaling of request body
//   - Setting headers (Content-Type, X-API-Key)
//   - Executing the HTTP request
//   - Reading and unmarshaling the response
//   - Error handling for non-2xx status codes
func (c *HTTPClient) doRequest(ctx context.Context, method, endpoint string, body interface{}, result interface{}) error {
	url := c.baseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Unmarshal response if result is provided
	if result != nil {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
