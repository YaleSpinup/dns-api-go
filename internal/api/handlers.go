/*
Copyright © 2023 Yale University

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/
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
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// PingHandler responds to ping requests
func (s *server) PingHandler(w http.ResponseWriter, r *http.Request) {
	w = LogWriter{w}
	logger.Debug("Ping/Pong")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

// VersionHandler responds to version requests
func (s *server) VersionHandler(w http.ResponseWriter, r *http.Request) {
	w = LogWriter{w}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	data, err := json.Marshal(s.version)
	if err != nil {
		logger.Error("Failed to marshal version data", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte{})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *server) SystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	body, err := s.MakeRequest("GET", "/getSystemInfo", "")
	if err != nil {
		logger.Error("Failed to retrieve system info",
			zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the response body into a map
	info := make(map[string]string)
	pairs := strings.Split(string(body), "|")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) == 2 {
			info[kv[0]] = kv[1]
		}
	}

	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")

	// Encode the map as JSON and write it to the response
	json.NewEncoder(w).Encode(info)
}

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
	body, err := ioutil.ReadAll(resp.Body)
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

func (s *server) MakeRequest(method, route, queryParam string) ([]byte, error) {
	// Construct the API URL
	apiURL := s.bluecat.baseUrl + route
	if queryParam != "" {
		apiURL += "?" + queryParam
	}
	token, err := s.getToken()
	logger.Debug("API URL", zap.String("URL", apiURL))

	// Create a new HTTP request
	req, err := http.NewRequest(strings.ToUpper(method), apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %v", err)
	}

	req.Header.Set("Authorization", token)

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
	body, err := io.ReadAll(resp.Body)
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

		return s.MakeRequest(method, route, queryParam)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("Unexpected status code received from API",
			zap.Int("StatusCode", resp.StatusCode),
			zap.String("Body", string(body)))
		return nil, fmt.Errorf("unexpected status code: %d, Body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (s *server) GetRecordHintHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	count := r.URL.Query().Get("count")
	options := r.URL.Query().Get("options")
	start := r.URL.Query().Get("start")
	recordType := r.URL.Query().Get("type")

	// Determine the API endpoint based on the record type
	var endpoint string
	switch recordType {
	case "HostRecord":
		endpoint = "/getHostRecordsByHint"
	case "AliasRecord":
		endpoint = "/getAliasesByHint"
	default:
		supportedTypes := []string{"HostRecord", "AliasRecord"}
		errorMsg := fmt.Sprintf("Invalid record type. Supported types: %s", strings.Join(supportedTypes, ", "))
		logger.Error("Invalid record type requested",
			zap.String("recordType", recordType),
			zap.Strings("supportedTypes", supportedTypes))
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Construct the query parameter string
	queryParam := fmt.Sprintf("count=%s&options=%s&start=%s", count, options, start)

	// Make the API request
	body, err := s.MakeRequest("GET", endpoint, queryParam)
	if err != nil {
		logger.Error("Failed to make API request for record hint",
			zap.String("endpoint", endpoint),
			zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")

	// Write the response body as-is
	w.Write(body)
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
