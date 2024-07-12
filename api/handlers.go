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
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"net/http"
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

func (s *server) makeRequest(route, queryParam string) ([]byte, error) {
	// Construct the API URL
	apiURL := s.bluecat.baseUrl + route
	if queryParam != "" {
		apiURL += "?" + queryParam
	}
	token, err := s.getToken()
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
	if resp.StatusCode == http.StatusUnauthorized {
		log.Printf("Unauthorized: Token expired or invalid. Generating a new token.")

		// Clear the current token
		s.bluecat.tokenLock.Lock()
		s.bluecat.token = ""
		s.bluecat.tokenLock.Unlock()

		return s.makeRequest(route, queryParam)
	}

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
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Construct the query parameter string
	queryParam := fmt.Sprintf("count=%s&options=%s&start=%s", count, options, start)

	// Make the API request
	body, err := s.makeRequest(endpoint, queryParam)
	if err != nil {
		log.Printf("Error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")

	// Write the response body as-is
	w.Write(body)
}

func (s *server) EntityIdHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the account and entity ID from the URL
	vars := mux.Vars(r)
	account := vars["account"]
	id := vars["id"]
	includeHA := r.URL.Query().Get("includeHA")
	if includeHA == "" {
		includeHA = "true"
	}

	// Validate the account
	if s.bluecat.account != account {
		http.Error(w, "Invalid account", http.StatusBadRequest)
		return
	}

	// Send http request to bluecat
	route, params := "/getEntityById", fmt.Sprintf("id=%s&includeHA=%s", id, includeHA)
	resp, err := s.makeRequest(route, params)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// return response
	w.Header().Set("Content-Type", "application/json")
	n, err := w.Write(resp)
	if err != nil {
		log.Printf("Failed to write response: %v, bytes written: %d", err, n)
	}
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
