package api

import (
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"encoding/json"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

// GetEntityByIdHandler handles GET requests for retrieving an entity by ID.
func (s *server) GetEntityByIdHandler(w http.ResponseWriter, r *http.Request) {
	// Extract entity ID parameter from the request URL
	vars := mux.Vars(r)
	idStr, idOk := vars["id"]

	// Validate the presence of the required 'id' parameter
	if !idOk {
		logger.Warn("Missing required parameter: id",
			zap.String("method", r.Method))
		http.Error(w, "Missing required parameter: id", http.StatusBadRequest)
		return
	}
	// Convert id from string to int
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logger.Error("Invalid ID format",
			zap.String("id", idStr),
			zap.Error(err))
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	// Default includeHA to "true" if not specified
	includeHAStr := r.URL.Query().Get("includeHA")
	includeHA, err := strconv.ParseBool(includeHAStr)
	if err != nil {
		includeHA = true
	}

	// Attempt to retrieve the entity by ID and handle potential errors
	entity, err := s.services.BaseService.GetEntityByID(id, includeHA)
	if err != nil {
		// Log the error and respond with appropriate HTTP status
		logger.Error("Error retrieving entity by ID",
			zap.Int("id", id),
			zap.Bool("includeHA", includeHA),
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

	// Serialize the entity to JSON and send it in the response
	jsonResponse, err := json.Marshal(entity)
	if err != nil {
		// Log the error and respond with an internal server error status
		logger.Error("Failed to marshal entity into JSON", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Successfully retrieved and marshaled entity; sending back to client
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonResponse); err != nil {
		// Log failure to write the response
		logger.Error("Failed to write response", zap.Error(err))
	}
}

// DeleteEntityByIdHandler handles DELETE requests for deleting an entity by ID.
func (s *server) DeleteEntityByIdHandler(w http.ResponseWriter, r *http.Request) {
	// Extract entity ID parameter from the request URL
	vars := mux.Vars(r)
	idStr, idOk := vars["id"]

	// Validate the presence of the required 'id' parameter
	if !idOk {
		logger.Warn("Missing required parameter: id",
			zap.String("method", r.Method))
		http.Error(w, "Missing required parameter: id", http.StatusBadRequest)
		return
	}
	// Convert id from string to int
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logger.Error("Invalid ID format",
			zap.String("id", idStr),
			zap.Error(err))
		http.Error(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	// Attempt to delete the entity by ID and handle potential errors
	err = s.services.BaseService.DeleteEntityByID(id)
	if err != nil {
		// Log the error and respond with appropriate HTTP status
		logger.Error("Error deleting entity by ID",
			zap.Int("id", id),
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
	w.WriteHeader(http.StatusNoContent)
}
