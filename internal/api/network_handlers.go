package api

import (
	"net/http"
)

// GetNetworksHandler retrieves networks from BlueCat using hint-based search
// @Summary Get networks by hint
// @Description Retrieves a list of IPv4 networks from BlueCat based on a search hint with pagination support
// @Tags Network Management
// @Produce json
// @Param account path string true "Account identifier"
// @Param hint query string false "Search hint to filter networks (e.g., network name or IP range)"
// @Param offset query int false "Number of records to skip for pagination" default(0)
// @Param limit query int false "Maximum number of records to return" default(10)
// @Success 200 {array} models.Entity "List of network entities"
// @Failure 400 "Invalid request parameters"
// @Failure 500 "Internal server error"
// @Router /{account}/networks [get]
func (s *server) GetNetworksHandler() http.HandlerFunc {
	return s.HandleGetEntitiesByHintReq(s.services.NetworkService)
}

// GetNetworkHandler retrieves a specific network by ID
// @Summary Get network by ID
// @Description Retrieves detailed information about a specific IPv4 network using its unique identifier
// @Tags Network Management
// @Produce json
// @Param account path string true "Account identifier"
// @Param id path int true "Network ID"
// @Param includeHA query bool false "Include high availability information" default(true)
// @Success 200 {object} models.Entity "Network details"
// @Failure 400 "Invalid request parameters"
// @Failure 404 "Network not found"
// @Failure 500 "Internal server error"
// @Router /{account}/networks/{id} [get]
func (s *server) GetNetworkHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.NetworkService)
}
