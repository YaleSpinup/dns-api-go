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
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *server) routes() {
	api := s.router.PathPrefix("/v2/dns").Subrouter()
	api.HandleFunc("/ping", s.PingHandler).Methods(http.MethodGet)
	api.HandleFunc("/version", s.VersionHandler).Methods(http.MethodGet)
	api.Handle("/metrics", promhttp.Handler()).Methods(http.MethodGet)
	api.HandleFunc("/", s.ProxyRequestHandler).Methods(http.MethodGet)
	api.HandleFunc("/systeminfo", s.SystemInfoHandler).Methods(http.MethodGet)
	api.HandleFunc("/record/hint", s.GetRecordHintHandler).Methods(http.MethodGet)

	// Create a subrouter for routes that need account validation
	accountRouter := api.PathPrefix("/{account}").Subrouter()

	// Apply the middleware to all routes in this subrouter
	accountRouter.Use(s.AccountValidationMiddleware)

	// Custom search based on type and filters
	api.HandleFunc("/search", s.ProxyRequestHandler).Methods(http.MethodGet)

	// Manage entities by ID
	accountRouter.HandleFunc("/{account}/id/{id}", s.GetEntityByIdHandler).Methods(http.MethodGet)
	accountRouter.HandleFunc("/{account}/id/{id}", s.DeleteEntityByIdHandler).Methods(http.MethodDelete)

	// Manage Zones
	api.HandleFunc("/{account}/zones", s.ProxyRequestHandler).Methods(http.MethodGet)
	api.HandleFunc("/{account}/zones/{id}", s.ProxyRequestHandler).Methods(http.MethodGet)

	// Manage DNS records
	api.HandleFunc("/{account}/records", s.ProxyRequestHandler).Methods(http.MethodGet, http.MethodPost)
	api.HandleFunc("/{account}/records/{id}", s.ProxyRequestHandler).Methods(http.MethodGet, http.MethodPut, http.MethodDelete)

	// Manage Networks
	api.HandleFunc("/{account}/networks", s.ProxyRequestHandler).Methods(http.MethodGet)
	api.HandleFunc("/{account}/networks/{id}", s.ProxyRequestHandler).Methods(http.MethodGet)

	// Manage IP addresses
	api.HandleFunc("/{account}/ips", s.ProxyRequestHandler).Methods(http.MethodPost)
	api.HandleFunc("/{account}/ips/{ip}", s.ProxyRequestHandler).Methods(http.MethodGet, http.MethodPut, http.MethodDelete)
	api.HandleFunc("/{account}/ips/cidrs", s.ProxyRequestHandler).Methods(http.MethodGet)

	// Manage MAC addresses
	api.HandleFunc("/{account}/macs", s.ProxyRequestHandler).Methods(http.MethodPost, http.MethodPut)
	api.HandleFunc("/{account}/macs/{mac}", s.ProxyRequestHandler).Methods(http.MethodGet)
}
