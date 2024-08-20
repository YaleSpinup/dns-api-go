package api

import (
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

type GetZonesParams struct {
	offset int
	limit  int
	hint   string
}

type GetZoneParams struct {
	zoneId    int
	IncludeHA bool
}

func parseGetZonesParams(r *http.Request) (*GetZonesParams, error) {
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

	return &GetZonesParams{
		offset: offset,
		limit:  limit,
		hint:   hint,
	}, nil
}

func parseGetZoneParams(r *http.Request) (*GetZoneParams, error) {
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

	return &GetZoneParams{
		zoneId:    id,
		IncludeHA: includeHA,
	}, nil
}

func (s *server) GetZonesHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the entity parameters from the request
	params, err := parseGetZonesParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Call the service to get zones
	zones, err := s.services.ZoneService.GetZones(params.offset, params.limit, map[string]string{"hint": params.hint})
	if err != nil {
		logger.Error("Failed to get zones", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Serialize the zones to JSON
	jsonResponse, err := json.Marshal(zones)
	if err != nil {
		logger.Error("Failed to marshal zones into JSON", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set the response headers and write the JSON response
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonResponse); err != nil {
		logger.Error("Failed to write response", zap.Error(err))
	}
}

func (s *server) GetZoneHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the entity parameters from the request
	params, err := parseGetZoneParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Call the service to get the zone
	zone, err := s.services.ZoneService.GetZone(params.zoneId, params.IncludeHA)
	if err != nil {
		// Log the error and respond with appropriate HTTP status
		logger.Error("Error retrieving zone entity",
			zap.Int("id", params.zoneId),
			zap.Bool("includeHA", params.IncludeHA),
			zap.String("method", r.Method),
			zap.Error(err))

		// Determine the type of error and set the HTTP response accordingly
		switch e := err.(type) {
		case *services.ErrEntityNotFound:
			http.Error(w, e.Error(), http.StatusNotFound)
			return
		case *services.ErrEntityTypeMismatch:
			http.Error(w, e.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Serialize the zone to JSON
	jsonResponse, err := json.Marshal(zone)
	if err != nil {
		logger.Error("Failed to marshal zone into JSON", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set the response headers and write the JSON response
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonResponse); err != nil {
		logger.Error("Failed to write response", zap.Error(err))
	}
}
