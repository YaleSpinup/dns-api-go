package api

import (
	"bytes"
	"crypto/tls"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"github.com/YaleSpinup/apierror"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// bluecatHTTPClient builds an http.Client matching the existing TLS posture
// (Yale BlueCat presents a self-signed cert; v1 also skipped verification).
func bluecatHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// generateAuthToken opens a BlueCat v2 session and returns the pre-encoded
// basicAuthenticationCredentials suitable for an Authorization: Basic header.
// As a side effect it stashes the session ID on s.bluecat so logout() can
// target it. Callers must hold s.bluecat.tokenLock.
func (s *server) generateAuthToken(username, password string) (string, error) {
	loginURL := s.bluecat.baseUrl + "/api/v2/sessions"
	logger.Debug("Login URL", zap.String("URL", loginURL))

	payload, err := json.Marshal(map[string]string{
		"type":     "UserSession",
		"username": username,
		"password": password,
	})
	if err != nil {
		return "", fmt.Errorf("marshal login payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, loginURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/hal+json")

	resp, err := bluecatHTTPClient().Do(req)
	if err != nil {
		logger.Error("Error sending login request", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading login response body", zap.Error(err))
		return "", err
	}

	if resp.StatusCode != http.StatusCreated {
		logger.Error("Login failed with status code",
			zap.Int("StatusCode", resp.StatusCode),
			zap.String("Body", string(body)))
		return "", fmt.Errorf("login failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var out struct {
		ID                             int    `json:"id"`
		APIToken                       string `json:"apiToken"`
		BasicAuthenticationCredentials string `json:"basicAuthenticationCredentials"`
		State                          string `json:"state"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("decode login response: %w; body: %s", err, string(body))
	}
	if out.BasicAuthenticationCredentials == "" {
		return "", fmt.Errorf("login response missing basicAuthenticationCredentials; body: %s", string(body))
	}

	s.bluecat.sessionID = out.ID
	logger.Debug("Opened v2 session", zap.Int("sessionID", out.ID), zap.String("state", out.State))

	return out.BasicAuthenticationCredentials, nil
}

// logout closes the active BlueCat v2 session. Safe to call when no session
// is open; returns nil in that case. Errors are logged but not surfaced to
// callers since logout is best-effort.
func (s *server) logout() error {
	s.bluecat.tokenLock.Lock()
	defer s.bluecat.tokenLock.Unlock()

	if s.bluecat.sessionID == 0 || s.bluecat.token == "" {
		return nil
	}

	req, err := http.NewRequest(http.MethodPatch,
		s.bluecat.baseUrl+"/api/v2/sessions/current",
		strings.NewReader(`{"state":"LOGGED_OUT"}`))
	if err != nil {
		return fmt.Errorf("build logout request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+s.bluecat.token)
	req.Header.Set("Content-Type", "application/merge-patch+json")
	req.Header.Set("Accept", "application/hal+json")

	resp, err := bluecatHTTPClient().Do(req)
	if err != nil {
		logger.Warn("logout request failed", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		logger.Warn("logout returned non-2xx",
			zap.Int("StatusCode", resp.StatusCode),
			zap.String("Body", string(body)))
	}

	s.bluecat.token = ""
	s.bluecat.sessionID = 0
	return nil
}

func (s *server) getToken() (string, error) {
	s.bluecat.tokenLock.Lock()
	defer s.bluecat.tokenLock.Unlock()

	if s.bluecat.token == "" {
		token, err := s.generateAuthToken(s.bluecat.user, s.bluecat.password)
		if err != nil {
			return "", err
		}
		s.bluecat.token = token
	}

	return s.bluecat.token, nil
}

func (s *server) MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error) {
	// Construct the API URL
	apiURL := s.bluecat.baseUrl + route
	if queryParam != "" {
		apiURL += "?" + queryParam
	}
	token, err := s.getToken()
	logger.Debug("API URL", zap.String("URL", apiURL))

	// Create a new HTTP request
	req, err := http.NewRequest(strings.ToUpper(method), apiURL, body)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %v", err)
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json") // Set Content-Type header

	// Send the HTTP request
	client := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	// Check the response status code
	if resp.StatusCode == http.StatusUnauthorized {
		logger.Warn("Unauthorized: Token expired or invalid. Generating a new token.",
			zap.String("route", route),
			zap.String("queryParam", queryParam))

		// Clear the current token
		s.bluecat.tokenLock.Lock()
		s.bluecat.token = ""
		s.bluecat.tokenLock.Unlock()

		return s.MakeRequest(method, route, queryParam, body)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("Unexpected status code received from API",
			zap.Int("StatusCode", resp.StatusCode),
			zap.String("Body", string(respBody)))
		return nil, fmt.Errorf("unexpected status code: %d, Body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// respond writes the response to the client
// adds a newline to the end of the response body
func (s *server) respond(w http.ResponseWriter, data interface{}, status int) {
	w.WriteHeader(status)

	if data != nil {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(data)
		if err != nil {
			// Log failure to write the response
			logger.Error("Failed to write response", zap.Error(err))
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	}
}

// handleError handles standard apierror return codes
func handleError(w http.ResponseWriter, err error) {
	logger.Error("API error", zap.Error(err))
	if aerr, ok := errors.Cause(err).(apierror.Error); ok {
		switch aerr.Code {
		case apierror.ErrForbidden:
			w.WriteHeader(http.StatusForbidden)
		case apierror.ErrNotFound:
			w.WriteHeader(http.StatusNotFound)
		case apierror.ErrConflict:
			w.WriteHeader(http.StatusConflict)
		case apierror.ErrBadRequest:
			w.WriteHeader(http.StatusBadRequest)
		case apierror.ErrLimitExceeded:
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(aerr.Message))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}

// validateMacAddress validates the format of the MAC address
// mac address should be in the format: nnnnnnnnnnnn or nn:nn:nn:nn:nn:nn or nn-nn-nn-nn-nn-nn
func validateMacAddress(macAddress string) error {
	// Define the regular expression for a valid MAC address
	macRegex := `^([0-9A-Fa-f]{12}|([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2})$`
	re := regexp.MustCompile(macRegex)

	// Validate the MAC address format
	if !re.MatchString(macAddress) {
		return fmt.Errorf("invalid MAC address format '%s'. MAC address should be in the format: nnnnnnnnnnnn or nn:nn:nn:nn:nn:nn or nn-nn-nn-nn-nn-nn", macAddress)
	}

	return nil
}
