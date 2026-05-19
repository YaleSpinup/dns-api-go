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
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"net/url"
)

func (s *server) HomeHandler(w http.ResponseWriter, _ *http.Request) {
	account := []string{s.bluecat.account}
	s.respond(w, account, http.StatusOK)
}

// PingHandler responds to ping requests
func (s *server) PingHandler(w http.ResponseWriter, _ *http.Request) {
	logger.Debug("Ping/Pong")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.respond(w, "pong", http.StatusOK)
}

// VersionHandler responds to version requests
func (s *server) VersionHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	s.respond(w, s.version, http.StatusOK)
}

func (s *server) SystemInfoHandler(w http.ResponseWriter, _ *http.Request) {
	query := "filter=" + url.QueryEscape("type:eq('SystemSettings')")
	body, err := s.MakeRequest("GET", "/api/v2/settings", query, nil)
	if err != nil {
		logger.Error("Failed to retrieve system info", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var page struct {
		Count int                      `json:"count"`
		Data  []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		logger.Error("Failed to decode system info response", zap.Error(err))
		http.Error(w, "invalid system info response", http.StatusInternalServerError)
		return
	}
	if len(page.Data) == 0 {
		logger.Error("System info response had no SystemSettings entry")
		http.Error(w, "system info unavailable", http.StatusInternalServerError)
		return
	}

	// Flatten the v2 entity to map[string]string (matching v1's contract).
	// _links is HAL plumbing, not info — drop it.
	info := make(map[string]string, len(page.Data[0]))
	for k, v := range page.Data[0] {
		if k == "_links" {
			continue
		}
		info[k] = fmt.Sprintf("%v", v)
	}

	s.respond(w, info, http.StatusOK)
}
