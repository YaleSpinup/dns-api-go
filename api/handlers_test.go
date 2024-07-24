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
	"dns-api-go/common"
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
