package api

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/services"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"strings"
)

type CustomSearchParams struct {
	filters    map[string]string
	objectType string
}

func parseCustomSearchParams(r *http.Request) (*CustomSearchParams, error) {
	var Params CustomSearchParams

	query := r.URL.Query()

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

// GetEntityHandler handles GET requests for retrieving an entity by ID.
func (s *server) GetEntityHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.BaseService)
}

// DeleteEntityHandler handles DELETE requests for deleting an entity by ID.
func (s *server) DeleteEntityHandler() http.HandlerFunc {
	return s.HandleDeleteEntityReq(s.services.BaseService)
}

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
	entities, err := s.services.BaseService.CustomSearch(0, 100, params.filters, nil, params.objectType)
	if err != nil {
		logger.Error("Error with custom search", zap.Error(err))
		// Determine thet ype of error and set the HTTP response accordingly
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
