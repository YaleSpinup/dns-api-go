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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

	expected := `"pong"`

	if strings.TrimSpace(rr.Body.String()) != expected {
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
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestSystemInfoHandler_DecodesV2Settings(t *testing.T) {
	var capturedQuery string
	var capturedPath string

	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"count": 1,
			"data": [{
				"id": 1,
				"type": "SystemSettings",
				"hostname": "BAM-3000",
				"version": "25.1.1-1157.GA.bcn",
				"address": "10.16.8.40",
				"interfaceRedundancyEnabled": false,
				"activeSessionCount": 3,
				"_links": {"self": {"href": "/api/v2/settings/1"}}
			}]
		}`))
	})
	defer ts.Close()

	req, _ := http.NewRequest("GET", "/v2/dns/systeminfo", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(s.SystemInfoHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if capturedPath != "/api/v2/settings" {
		t.Errorf("path = %s, want /api/v2/settings", capturedPath)
	}
	if !strings.Contains(capturedQuery, "filter=type%3Aeq") {
		t.Errorf("query = %s, want URL-encoded filter on type:eq", capturedQuery)
	}
	if !strings.Contains(capturedQuery, "SystemSettings") {
		t.Errorf("query = %s, want filter value to mention SystemSettings", capturedQuery)
	}

	var info map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, rr.Body.String())
	}
	if info["hostname"] != "BAM-3000" {
		t.Errorf("hostname = %q, want BAM-3000", info["hostname"])
	}
	if info["version"] != "25.1.1-1157.GA.bcn" {
		t.Errorf("version = %q, want 25.1.1-1157.GA.bcn", info["version"])
	}
	if info["interfaceRedundancyEnabled"] != "false" {
		t.Errorf("interfaceRedundancyEnabled = %q, want stringified bool 'false'", info["interfaceRedundancyEnabled"])
	}
	if info["activeSessionCount"] != "3" {
		t.Errorf("activeSessionCount = %q, want stringified int '3'", info["activeSessionCount"])
	}
	if _, present := info["_links"]; present {
		t.Error("_links should be dropped from response")
	}
}

func TestSystemInfoHandler_EmptyDataIsError(t *testing.T) {
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"count":0,"data":[]}`))
	})
	defer ts.Close()

	req, _ := http.NewRequest("GET", "/v2/dns/systeminfo", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(s.SystemInfoHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when SystemSettings entry missing", rr.Code)
	}
}

func TestSystemInfoHandler_UpstreamErrorPropagates(t *testing.T) {
	ts, s, _ := makeRequestRouter(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"message":"upstream down"}`))
	})
	defer ts.Close()

	req, _ := http.NewRequest("GET", "/v2/dns/systeminfo", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(s.SystemInfoHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when upstream errors", rr.Code)
	}
}
