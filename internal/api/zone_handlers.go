package api

import (
	"net/http"
)

// GetZonesHandler retrieves DNS zones from BlueCat using hint-based search
// @Summary Get DNS zones by hint
// @Description Retrieves a list of DNS zones from BlueCat based on a search hint with pagination support
// @Tags Zone Management
// @Produce json
// @Param account path string true "Account identifier"
// @Param hint query string false "Search hint to filter zones (e.g., zone name or domain)"
// @Param offset query int false "Number of records to skip for pagination" default(0)
// @Param limit query int false "Maximum number of records to return" default(10)
// @Success 200 {array} models.Entity "List of zone entities"
// @Failure 400 "Invalid request parameters"
// @Failure 500 "Internal server error"
// @Router /{account}/zones [get]
func (s *server) GetZonesHandler() http.HandlerFunc {
	return s.HandleGetEntitiesByHintReq(s.services.ZoneService)
}

// GetZoneHandler retrieves a specific DNS zone by ID
// @Summary Get DNS zone by ID
// @Description Retrieves detailed information about a specific DNS zone using its unique identifier
// @Tags Zone Management
// @Produce json
// @Param account path string true "Account identifier"
// @Param id path int true "Zone ID"
// @Param includeHA query bool false "Include high availability information" default(true)
// @Success 200 {object} models.Entity "Zone details"
// @Failure 400 "Invalid request parameters"
// @Failure 404 "Zone not found"
// @Failure 500 "Internal server error"
// @Router /{account}/zones/{id} [get]
func (s *server) GetZoneHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.ZoneService)
}
