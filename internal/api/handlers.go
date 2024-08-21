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
	"dns-api-go/logger"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"strings"
)

// PingHandler responds to ping requests
func (s *server) PingHandler(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Ping/Pong")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.respond(w, "pong", http.StatusOK)
}

// VersionHandler responds to version requests
func (s *server) VersionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.respond(w, s.version, http.StatusOK)
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

	// Encode the map as JSON and write it to the response
	s.respond(w, info, http.StatusOK)
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
