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
	"net/http"
	"strings"

	"go.uber.org/zap"
)

// HomeHandler returns the configured BlueCat account information
// @Summary Get account information
// @Description Returns the BlueCat account configured for this DNS API instance
// @Tags General
// @Produce json
// @Success 200 {array} string "List containing the configured account name"
// @Router / [get]
func (s *server) HomeHandler(w http.ResponseWriter, _ *http.Request) {
	account := []string{s.bluecat.account}
	s.respond(w, account, http.StatusOK)
}

// PingHandler responds to health check requests
// @Summary Health check endpoint
// @Description Performs a basic health check to verify the API is running
// @Tags Health
// @Produce json
// @Success 200 "Returns 'pong' to indicate the service is healthy"
// @Router /ping [get]
func (s *server) PingHandler(w http.ResponseWriter, _ *http.Request) {
	logger.Debug("Ping/Pong")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.respond(w, "pong", http.StatusOK)
}

// VersionHandler responds to version requests
// @Summary Get API version information
// @Description Returns version, build timestamp, and git commit information for the API
// @Tags General
// @Produce json
// @Success 200 {object} apiVersion "API version information"
// @Router /version [get]
func (s *server) VersionHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.respond(w, s.version, http.StatusOK)
}

// SystemInfoHandler retrieves Bluecat system information
// @Summary Get BlueCat system information
// @Description Retrieves system information from the connected BlueCat instance
// @Tags System
// @Produce json
// @Success 200 {object} map[string]string "BlueCat system information as key-value pairs"
// @Failure 500 "Internal server error if the request fails"
// @Router /systeminfo [get]
func (s *server) SystemInfoHandler(w http.ResponseWriter, _ *http.Request) {
	body, err := s.MakeRequest("GET", "/getSystemInfo", "", nil)
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
