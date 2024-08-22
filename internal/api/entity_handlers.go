package api

import (
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"go.uber.org/zap"
	"net/http"
)

// GetEntityHandler handles GET requests for retrieving an entity by ID.
func (s *server) GetEntityHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.BaseService)
}

// DeleteEntityHandler handles DELETE requests for deleting an entity by ID.
func (s *server) DeleteEntityHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the entity parameters from the request
	params, err := parseEntityParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Attempt to delete the entity by ID and handle potential errors
	err = s.services.BaseService.DeleteEntity(params.ID, []string{})
	if err != nil {
		// Log the error and respond with appropriate HTTP status
		logger.Error("Error deleting entity by ID",
			zap.Int("id", params.ID),
			zap.String("method", r.Method),
			zap.Error(err))

		// Determine the type of error and set the HTTP response accordingly
		switch e := err.(type) {
		case *services.ErrDeleteNotAllowed:
			http.Error(w, e.Error(), http.StatusNotFound)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Successfully deleted the entity; acknowledging with no content status
	s.respond(w, nil, http.StatusNoContent)
}
