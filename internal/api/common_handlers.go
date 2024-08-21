package api

import (
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"fmt"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

// EntityParams represents the parameters for entity handlers.
type EntityParams struct {
	ID        int
	IncludeHA bool
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

// HandleGetEntityReq returns an HTTP handler function that processes requests to retrieve an entity by ID.
// It uses the provided EntityGetter interface to fetch the entity and handles various error scenarios.
func (s *server) HandleGetEntityReq(service services.EntityGetter) http.HandlerFunc {
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
