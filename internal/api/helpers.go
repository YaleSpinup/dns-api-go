package api

import (
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

func (s *server) generateAuthToken(username, password string) (string, error) {
	// Construct the login URL
	loginURL := fmt.Sprintf("%s/login?username=%s&password=%s", s.bluecat.baseUrl, username, password)
	logger.Debug("Login URL", zap.String("URL", loginURL))

	client := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Send the login request using the custom http.Client
	resp, err := client.Get(loginURL)
	if err != nil {
		logger.Error("Error sending login request", zap.Error(err))
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Error reading login response body", zap.Error(err))
		return "", err
	}

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		logger.Error("Login failed with status code",
			zap.Int("StatusCode", resp.StatusCode),
			zap.String("Body", string(body)))
		return "", fmt.Errorf("login failed: %s", string(body))
	}

	// Extract the token from the response body
	token := strings.TrimPrefix(string(body), "\"Session Token-> ")
	token = strings.TrimSuffix(token, " <- for User : "+username+"\"")
	logger.Debug("Generated authentication token", zap.String("Token", token))

	return token, nil
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
		return fmt.Errorf("invalid MAC address format. MAC address should be in the format: nnnnnnnnnnnn or nn:nn:nn:nn:nn:nn or nn-nn-nn-nn-nn-nn")
	}

	return nil
}
