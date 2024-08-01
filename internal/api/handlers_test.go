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
	"dns-api-go/internal/common"
	"dns-api-go/internal/models"
	"dns-api-go/internal/services"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Setup phase: Initialize the logger
	common.SetupLogger()

	// Run the tests
	code := m.Run()

	// Exit with the code from m.Run()
	os.Exit(code)
}

func TestPingHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/v1/test/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s := server{}
	handler := http.HandlerFunc(s.PingHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `pong`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestVersionHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/v1/test/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	s := server{
		version: &apiVersion{
			Version:    "0.1.0",
			GitHash:    "No Git Commit Provided",
			BuildStamp: "No BuildStamp Provided",
		},
	}
	handler := http.HandlerFunc(s.VersionHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"version":"0.1.0","githash":"No Git Commit Provided","buildstamp":"No BuildStamp Provided"}`
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestGetEntityIDHandler(t *testing.T) {
	tests := []struct {
		name           string
		id             string
		includeHA      string
		mockEntity     *models.Entity
		mockError      error
		expectedBody   string
		expectedStatus int
	}{
		{
			name:      "Successful retrieval",
			id:        "1",
			includeHA: "true",
			mockEntity: &models.Entity{
				ID:         1,
				Name:       "Test Entity",
				Type:       "TestType",
				Properties: "TestProperties",
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"ID":1,"Name":"Test Entity","Type":"TestType","Properties":"TestProperties"}`,
		},
		{
			name:      "Missing includeHA",
			id:        "1",
			includeHA: "",
			mockEntity: &models.Entity{
				ID:         1,
				Name:       "Test Entity",
				Type:       "TestType",
				Properties: "TestProperties",
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"ID":1,"Name":"Test Entity","Type":"TestType","Properties":"TestProperties"}`,
		},
		{
			name:           "Invalid ID format",
			id:             "invalid",
			includeHA:      "true",
			mockEntity:     nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid ID format\n",
		},
		{
			name:           "Entity not found",
			id:             "999",
			includeHA:      "true",
			mockEntity:     nil,
			mockError:      &services.ErrEntityNotFound{},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Entity not found\n",
		},
		{
			name:           "GetEntityByID server error",
			id:             "2",
			includeHA:      "true",
			mockEntity:     nil,
			mockError:      errors.New("internal error"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal error\n",
		},
		{
			name:           "Marshaling error",
			id:             "1",
			includeHA:      "true",
			mockEntity:     &models.Entity{},
			mockError:      nil,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal server error\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// TODO: Implement the test
		})
	}
}
