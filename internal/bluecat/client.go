package bluecat

import (
	"bytes"
	"crypto/tls"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client is a Bluecat Address Manager V2 REST API client.
type Client struct {
	baseURL    string
	username   string
	password   string
	token      string
	tokenLock  sync.Mutex
	httpClient *http.Client
}

// NewClient creates a new Bluecat V2 API client.
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

// Authenticate obtains a bearer token via POST /api/v2/sessions with Basic auth.
func (c *Client) Authenticate() (string, error) {
	url := c.baseURL + "/api/v2/sessions"
	logger.Debug("Authenticating with Bluecat V2 API", zap.String("url", url))

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating auth request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Error("Error sending auth request", zap.Error(err))
		return "", fmt.Errorf("error sending auth request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading auth response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("Authentication failed",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("body", string(body)))
		return "", fmt.Errorf("authentication failed (status %d): %s", resp.StatusCode, string(body))
	}

	var sessionResp V2SessionResponse
	if err := json.Unmarshal(body, &sessionResp); err != nil {
		return "", fmt.Errorf("error parsing auth response: %w", err)
	}

	if sessionResp.APIToken == "" {
		return "", fmt.Errorf("empty apiToken in auth response")
	}

	logger.Debug("Successfully authenticated with Bluecat V2 API")
	return sessionResp.APIToken, nil
}

// getToken returns the cached token or authenticates to get a new one.
func (c *Client) getToken() (string, error) {
	c.tokenLock.Lock()
	defer c.tokenLock.Unlock()

	if c.token == "" {
		token, err := c.Authenticate()
		if err != nil {
			return "", err
		}
		c.token = token
	}
	return c.token, nil
}

// clearToken clears the cached token (called on 401).
func (c *Client) clearToken() {
	c.tokenLock.Lock()
	defer c.tokenLock.Unlock()
	c.token = ""
}


// Do executes an HTTP request against the V2 API.
// It handles token injection and auto-refresh on 401.
func (c *Client) Do(method, path string, body interface{}) ([]byte, error) {
	return c.doRequest(method, path, body, true)
}

// doRequest is the internal implementation that supports retry control.
func (c *Client) doRequest(method, path string, body interface{}, allowRetry bool) ([]byte, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}

	// Build the full URL
	url := c.baseURL + path
	logger.Debug("V2 API request", zap.String("method", method), zap.String("url", url))

	// Prepare request body
	var bodyReader io.Reader
	if body != nil {
		switch v := body.(type) {
		case io.Reader:
			bodyReader = v
		case []byte:
			bodyReader = bytes.NewReader(v)
		case string:
			bodyReader = strings.NewReader(v)
		default:
			jsonBytes, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("error marshalling request body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBytes)
		}
	}

	req, err := http.NewRequest(strings.ToUpper(method), url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Handle 401 - clear token and retry once
	if resp.StatusCode == http.StatusUnauthorized && allowRetry {
		logger.Warn("Unauthorized: Token expired or invalid. Refreshing token.",
			zap.String("path", path))
		c.clearToken()
		return c.doRequest(method, path, body, false)
	}

	// Handle 404 as not found (return nil body, no error — callers check for this)
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	// Handle other non-success status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Error("Unexpected status code from V2 API",
			zap.Int("statusCode", resp.StatusCode),
			zap.String("body", string(respBody)))
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// MakeRequest implements the ServerInterface-compatible method signature.
// The route parameter should be a V2 API path (e.g., "/api/v2/configurations").
// The queryParam is appended as a query string.
func (c *Client) MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error) {
	path := route
	if queryParam != "" {
		path += "?" + queryParam
	}
	return c.Do(method, path, body)
}

// GetCIDRFile is not implemented on the client — it's handled by the server.
// This stub exists to satisfy ServerInterface if needed.
func (c *Client) GetCIDRFile() (string, error) {
	return "", fmt.Errorf("GetCIDRFile is not available on the Bluecat client")
}
