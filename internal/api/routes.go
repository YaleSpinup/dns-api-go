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

	// Custom search based on type and filters
	api.HandleFunc("/search", s.ProxyRequestHandler).Methods(http.MethodGet)

	// Create a subrouter for routes that need account validation
	accountRouter := api.PathPrefix("/{account}").Subrouter()

	// Apply the middleware to all routes in this subrouter
	accountRouter.Use(s.AccountValidationMiddleware)

	// Manage entities by ID
	accountRouter.HandleFunc("/id/{id}", s.GetEntityHandler()).Methods(http.MethodGet)
	accountRouter.HandleFunc("/id/{id}", s.DeleteEntityHandler()).Methods(http.MethodDelete)

	// Manage Zones
	accountRouter.HandleFunc("/zones", s.GetZonesHandler()).Methods(http.MethodGet)
	accountRouter.HandleFunc("/zones/{id}", s.GetZoneHandler()).Methods(http.MethodGet)

	// Manage DNS records
	accountRouter.HandleFunc("/records", s.GetRecordsHandler).Methods(http.MethodGet)
	accountRouter.HandleFunc("/records/{id}", s.GetRecordHandler()).Methods(http.MethodGet)
	accountRouter.HandleFunc("/records/{id}", s.DeleteRecordHandler()).Methods(http.MethodDelete)
	accountRouter.HandleFunc("/records", s.CreateRecordHandler).Methods(http.MethodPost)

	// Manage Networks
	accountRouter.HandleFunc("/networks", s.GetNetworksHandler()).Methods(http.MethodGet)
	accountRouter.HandleFunc("/networks/{id}", s.GetNetworkHandler()).Methods(http.MethodGet)

	// Manage IP addresses
	accountRouter.HandleFunc("/ips/cidrs", s.GetCIDRHandler).Methods(http.MethodGet)
	accountRouter.HandleFunc("/ips/{ip}", s.GetIpAddressHandler).Methods(http.MethodGet)
	accountRouter.HandleFunc("/ips/{ip}", s.DeleteIpAddressHandler).Methods(http.MethodDelete)
	accountRouter.HandleFunc("/ips", s.AssignIpAddressHandler).Methods(http.MethodPost)

	// Manage MAC addresses
	accountRouter.HandleFunc("/macs/{mac}", s.GetMacAddressHandler).Methods(http.MethodGet)
	accountRouter.HandleFunc("/macs", s.CreateMacAddressHandler).Methods(http.MethodPost)
	accountRouter.HandleFunc("/macs/{mac}", s.UpdateMacAddressHandler).Methods(http.MethodPut)
}
