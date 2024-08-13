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
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestTokenMiddleware(t *testing.T) {
	psk := []byte("sometesttoken")
	tokenHeader, _ := bcrypt.GenerateFromPassword(psk, bcrypt.DefaultCost)

	// Test handler that just returns 200 OK
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Got request on %+v", r.URL)
		w.WriteHeader(http.StatusOK)
	})

	pubUrls := map[string]string{
		"/foo": "public",
		"/bar": "public",
		"/baz": "public",
	}

	// Start a new server with our token middleware and test handler
	server := httptest.NewServer(TokenMiddleware(psk, pubUrls, okHandler))
	defer server.Close()

	// Test some public urls
	for u := range pubUrls {
		url := fmt.Sprintf("%s%s", server.URL, u)
		t.Logf("Getting %s", url)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Received %d for public url '%s', expected %d", resp.StatusCode, u, http.StatusOK)
		}
	}

	// Test a bad URI
	_, err := http.Get(fmt.Sprintf("%s/\n", server.URL))
	if err == nil {
		t.Fatal("expected error for bad URL")
	}

	// Test a private URL without an auth token
	resp, err := http.Get(fmt.Sprintf("%s/private", server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Received status: %d for '%s/private', expected %d", resp.StatusCode, server.URL, http.StatusForbidden)
	}

	// Test a private URL _with_ an auth-token
	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/private", server.URL), nil)
	req.Header.Add("X-Auth-Token", string(tokenHeader))
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Received %d for '%s/private', expected %d", resp.StatusCode, server.URL, http.StatusOK)
	}

	// Test a private URL with options
	req, _ = http.NewRequest(http.MethodOptions, fmt.Sprintf("%s/optionstuff", server.URL), nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Received %d for '%s/optionstuff', expected %d", resp.StatusCode, server.URL, http.StatusOK)
	}

	testHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Headers": "X-Auth-Token",
	}

	for k, v := range testHeaders {
		if h, ok := resp.Header[k]; !ok || h[0] != v {
			t.Errorf("Expected response header %s from OPTIONS request to be %s, got %s", k, v, h[0])
		}
	}
}

func TestAccountValidationMiddleware(t *testing.T) {
	// Mock server setup
	mockServer := server{
		bluecat: &bluecat{
			account: "validAccount",
		},
	}

	// Mock next handler
	mockNextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Define tests
	tests := []struct {
		name           string
		account        string
		expectedStatus int
	}{
		{
			name:           "Valid account",
			account:        "validAccount",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid account",
			account:        "invalidAccount",
			expectedStatus: http.StatusBadRequest,
		},
	}

	// Run tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Crate new request
			req, err := http.NewRequest("GET", "/"+tc.account+"/someEndpoint", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Create a ResponseRecorder to record the response
			rr := httptest.NewRecorder()

			// Create a new router and apply the middleware
			router := mux.NewRouter()
			router.Use(mockServer.AccountValidationMiddleware)
			router.HandleFunc("/{account}/someEndpoint", mockNextHandler)

			// Server the request
			router.ServeHTTP(rr, req)

			// Check the status code
			if status := rr.Code; status != tc.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tc.expectedStatus)
			}
		})
	}
}
