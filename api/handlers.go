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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/YaleSpinup/apierror"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// PingHandler responds to ping requests
func (s *server) PingHandler(w http.ResponseWriter, r *http.Request) {
	w = LogWriter{w}
	log.Debug("Ping/Pong")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

func (s *server) generateAuthToken(username, password string) (string, error) {
	// Construct the login URL
	loginURL := fmt.Sprintf("%s/login?username=%s&password=%s", s.bluecat.baseUrl, username, password)
	log.Printf("Login URL: %s", loginURL)

	client := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Send the login request using the custom http.Client
	resp, err := client.Get(loginURL)
	if err != nil {
		log.Printf("Error sending login request: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading login response body: %v", err)
		return "", err
	}

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		log.Printf("Login failed with status code: %d, Body: %s", resp.StatusCode, string(body))
		return "", fmt.Errorf("login failed: %s", string(body))
	}

	// Extract the token from the response body
	token := strings.TrimPrefix(string(body), "\"Session Token-> ")
	token = strings.TrimSuffix(token, " <- for User : "+username+"\"")
	log.Printf("Generated authentication token: %s", token)

	return token, nil
}

func (s *server) makeRequest(route, queryParam string) ([]byte, error) {
	// Construct the API URL
	apiURL := s.bluecat.baseUrl + route
	if queryParam != "" {
		apiURL += "?" + queryParam
	}
	token, err := s.generateAuthToken(s.bluecat.user, s.bluecat.password)
	log.Printf("API URL: %s", apiURL)

	// Create a new HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, Body: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (s *server) SystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	body, err := s.makeRequest("/getSystemInfo", "")
	if err != nil {
		log.Printf("Error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Parse the response body
	info := make(map[string]string)
	pairs := strings.Split(string(body), "|")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) == 2 {
			info[kv[0]] = kv[1]
		}
	}

	// Create a JSON response
	jsonResp := make(map[string]interface{})
	jsonResp["hostName"] = getValueOrDefault(info, "hostName", "")
	jsonResp["version"] = getValueOrDefault(info, "version", "")
	jsonResp["address"] = getValueOrDefault(info, "address", "")
	jsonResp["clusterRole"] = getValueOrDefault(info, "clusterRole", "")
	jsonResp["replicationRole"] = getValueOrDefault(info, "replicationRole", "")
	jsonResp["replicationStatus"] = getValueOrDefault(info, "replicationStatus", "")
	jsonResp["entityCount"] = getValueOrDefault(info, "entityCount", "")
	jsonResp["databaseSize"] = getValueOrDefault(info, "databaseSize", "")
	jsonResp["loggedInUsers"] = getValueOrDefault(info, "loggedInUsers", "")

	// Send the JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jsonResp)
}

func getValueOrDefault(info map[string]string, key string, defaultValue string) string {
	if value, ok := info[key]; ok {
		return value
	}
	return defaultValue
}

func (s *server) SearchHandler(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Method: %s, URL: %s, Client IP: %s", r.Method, r.URL, r.RemoteAddr)

	token, err := s.generateAuthToken(s.bluecat.user, s.bluecat.password)

	// Parse the query parameters from the request
	query := r.URL.Query()

	// Extract the object type and field name=value pairs from the query parameters
	objectType := query.Get("type")
	fieldParams := make(map[string]string)
	for key, values := range query {
		if key != "type" && key != "X-Auth-Token" && len(values) > 0 {
			fieldParams[key] = values[0]
		}
	}

	// Construct the API request URL
	apiURL := s.bluecat.baseUrl + "/customSearch"

	// Create a new URL object with the API URL
	u, err := url.Parse(apiURL)
	if err != nil {
		log.Errorf("Failed to parse API URL: %s", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set the query parameters on the new URL object
	q := u.Query()
	q.Set("type", objectType)
	for key, value := range fieldParams {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	// Create a new HTTP request with the context from the incoming request and the new URL
	req, err := http.NewRequestWithContext(r.Context(), "GET", u.String(), nil)
	if err != nil {
		log.Errorf("Failed to create API request: %s", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set the necessary headers (e.g., authentication token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authentication", token)

	// Make the API request
	client := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Failed to make API request: %s", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		log.Errorf("API request failed with status code: %d %s %s", resp.StatusCode, apiURL, req.URL)
		http.Error(w, "Upstream server error", resp.StatusCode)
		return
	}

	// Parse the response body
	var searchResults interface{}
	err = json.NewDecoder(resp.Body).Decode(&searchResults)
	if err != nil {
		log.Errorf("Failed to parse API response: %s", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the search results as JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(searchResults)
}

// VersionHandler responds to version requests
func (s *server) VersionHandler(w http.ResponseWriter, r *http.Request) {
	w = LogWriter{w}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	data, err := json.Marshal(s.version)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte{})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// handleError handles standard apierror return codes
func handleError(w http.ResponseWriter, err error) {
	log.Error(err.Error())
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
