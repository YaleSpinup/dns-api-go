package api

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/services"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// CustomSearchParams represents the parameters for custom search requests
type CustomSearchParams struct {
	offset     int
	limit      int
	filters    map[string]string
	objectType string
}

func parseCustomSearchParams(r *http.Request) (*CustomSearchParams, error) {
	var Params CustomSearchParams

	// Set default values
	Params.offset = 0
	Params.limit = 100

	query := r.URL.Query()

	// Parse offset if it is not empty
	if offsetStr := query.Get("offset"); offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		// Return error if offset is not a valid integer
		if err != nil {
			return nil, fmt.Errorf("invalid offset value: %v", err)
		}
		// Return error if offset is negative
		if parsedOffset < 0 {
			return nil, fmt.Errorf("offset cannot be negative")
		}
		// Override the default value
		Params.offset = parsedOffset
	}

	// Parse limit if it is not empty
	if limitStr := query.Get("limit"); limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		// Return error if limit is not a valid integer
		if err != nil {
			return nil, fmt.Errorf("invalid limit value: %v", err)
		}
		// Return error if limit is negative
		if parsedLimit < 0 {
			return nil, fmt.Errorf("limit cannot be negative")
		}
		// Override the default value
		Params.limit = parsedLimit
	}

	// Parse filters from the request URL
	filters := query.Get("filters")
	if filters == "" {
		return nil, fmt.Errorf("missing required parameter: filters")
	}
	// Convert filters to map, and if empty, it means it was not formatted correctly
	Params.filters = common.ConvertToMap(filters, "|")
	if len(Params.filters) == 0 {
		return nil, fmt.Errorf("invalid filters format")
	}

	// Parse objectType from the request URL
	objectType := query.Get("type")
	if objectType == "" {
		return nil, fmt.Errorf("missing required parameter: type")
	}
	// Make sure the objectType is valid
	supportedTypes := []string{types.IP4BLOCK, types.IP4NETWORK, types.IP4ADDRESS, types.GENERICRECORD, types.HOSTRECORD}
	if !common.Contains(supportedTypes, objectType) {
		return nil, fmt.Errorf("invalid objectType: %s. Supported types are %s", objectType, strings.Join(supportedTypes, ", "))
	}
	Params.objectType = objectType

	return &Params, nil
}

// GetEntityHandler handles GET requests for retrieving an entity by ID
// @Summary Get entity by ID
// @Description Retrieves detailed information about a specific entity using its unique identifier
// @Tags Entity Management
// @Produce json
// @Param account path string true "Account identifier"
// @Param id path int true "Entity ID"
// @Param includeHA query bool false "Include high availability information" default(true)
// @Success 200 {object} models.Entity "Entity details"
// Failure 400 "Invalid request parameters"
// Failure 404 "Entity not found"
// Failure 500 "Internal server error"
// @Router /{account}/id/{id} [get]
func (s *server) GetEntityHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.BaseService)
}

// DeleteEntityHandler handles DELETE requests for deleting an entity by ID
// @Summary Delete entity by ID
// @Description Permanently deletes an entity from BlueCat. Only certain entity types can be deleted
// @Tags Entity Management
// @Param account path string true "Account identifier"
// @Param id path int true "Entity ID"
// @Success 204 "Entity deleted successfully"
// @Failure 400 "Invalid request parameters"
// @Failure 403 "Delete operation not allowed"
// @Failure 404 "Entity not found"
// @Failure 500 "Internal server error"
// @Router /{account}/id/{id} [delete]
func (s *server) DeleteEntityHandler() http.HandlerFunc {
	return s.HandleDeleteEntityReq(s.services.BaseService)
}

// CustomSearchHandler performs custom search across different entity types
// @Summary Custom entity search
// @Description Search for entities using flexible filters and type-specific criteria. Supports searching across IP blocks, networks, addresses, and DNS records
// @Tags Entity Management
// @Produce json
// @Param account path string true "Account identifier"
// @Param type query string true "Entity type to search for" Enums(IP4Block IP4Network, IP4Address, GenericRecord, HostRecord)
// @Param filters query string true "Search filters in format 'key1=value1|key2=value2'"
// @Param offset query int false "Number of records to skip for pagination" default(0)
// @Param limit query int false "Maximum number of records to return" default(100)
// @Success 200 {array} models.Entity "List of matching entities"
// @Failure 400 "Invalid request parameters or unsupported entity type"
// @Failure 404 "No entities found matching the search criteria"
// @Failure 500 "Internal server error"
// @Router /{account}/search [get]
func (s *server) CustomSearchHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("CustomSearchHandler started")

	// Parse parameters from the request
	params, err := parseCustomSearchParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Call base service
	entities, err := s.services.BaseService.CustomSearch(params.offset, params.limit, params.filters, nil, params.objectType)
	if err != nil {
		logger.Error("Error with custom search", zap.Error(err))
		// Determine the type of error and set the HTTP response accordingly
		switch e := err.(type) {
		case *services.ErrEntityNotFound:
			http.Error(w, e.Error(), http.StatusNotFound)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	logger.Info("CustomSearchHandler successful")
	s.respond(w, entities, http.StatusOK)
}
