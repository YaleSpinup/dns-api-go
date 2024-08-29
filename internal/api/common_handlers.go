package api

import (
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"fmt"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

// EntityParams represents the parameters for get entity handlers.
type EntityParams struct {
	ID        int
	IncludeHA bool
}

// EntitiesByHintParams represents the parameters for get entities by hint handlers.
type EntitiesByHintParams struct {
	offset int
	limit  int
	hint   string
}

// parseEntityParams parses and validates the parameters from the request.
func parseEntityParams(r *http.Request) (*EntityParams, error) {
	// Extract entity ID parameter from the request URL
	vars := mux.Vars(r)
	idStr, idOk := vars["id"]

	// Validate the presence of the required 'id' parameter
	if !idOk {
		return nil, fmt.Errorf("missing required parameter: id")
	}
	// Convert id from string to int
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ID format")
	}

	// Default includeHA to "true" if not specified
	includeHAStr := r.URL.Query().Get("includeHA")
	includeHA, err := strconv.ParseBool(includeHAStr)
	if err != nil {
		includeHA = true
	}

	return &EntityParams{
		ID:        id,
		IncludeHA: includeHA,
	}, nil
}

// parseEntitiesByHintParams parses and validates the parameters from the request.
func parseEntitiesByHintParams(r *http.Request) (*EntitiesByHintParams, error) {
	// Set default values
	offset := 0
	limit := 10
	hint := ""

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
		offset = parsedOffset
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
		limit = parsedLimit
	}

	// Parse hint if it is not empty
	if hintStr := query.Get("hint"); hintStr != "" {
		hint = hintStr
	}

	return &EntitiesByHintParams{
		offset: offset,
		limit:  limit,
		hint:   hint,
	}, nil
}

// HandleGetEntityReq returns an HTTP handler function that processes requests to retrieve an entity by ID.
// It uses the provided EntityGetter interface to fetch the entity and handles various error scenarios.
func (s *server) HandleGetEntityReq(service interfaces.EntityGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the entity parameters from the request
		params, err := parseEntityParams(r)
		if err != nil {
			logger.Warn("Invalid request parameters", zap.Error(err))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Attempt to retrieve the entity by ID and handle potential errors
		entity, err := service.GetEntity(params.ID, params.IncludeHA)
		if err != nil {
			// Log the error and respond with appropriate HTTP status
			logger.Error("Error retrieving entity by ID",
				zap.Int("id", params.ID),
				zap.Bool("includeHA", params.IncludeHA),
				zap.String("method", r.Method),
				zap.Error(err))

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

		// Successfully retrieved entity; sending back to client
		s.respond(w, entity, http.StatusOK)
	}
}

// HandleGetEntitiesByHintReq returns an HTTP handler function that processes requests to retrieve entities by hint.
// It uses the provided EntitiesLister interface to fetch the entities and handles various error scenarios.
func (s *server) HandleGetEntitiesByHintReq(service interfaces.EntitiesByHintLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse the entity parameters from the request
		params, err := parseEntitiesByHintParams(r)
		if err != nil {
			logger.Warn("Invalid request parameters", zap.Error(err))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Call the service to get entities by hint
		entities, err := service.GetEntitiesByHint(params.offset, params.limit, map[string]string{"hint": params.hint})
		if err != nil {
			logger.Error("Failed to get entities", zap.Error(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set the response headers and write the JSON response
		s.respond(w, entities, http.StatusOK)
	}

}
