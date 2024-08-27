package api

import (
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

type GetNetworksParams struct {
	offset int
	limit  int
	hint   string
}

func parseGetNetworksParams(r *http.Request) (*GetNetworksParams, error) {
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

	return &GetNetworksParams{
		offset: offset,
		limit:  limit,
		hint:   hint,
	}, nil
}

func (s *server) GetNetworksHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the entity parameters from the request
	params, err := parseGetNetworksParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Call the service to get networks
	networks, err := s.services.NetworkService.GetNetworks(params.offset, params.limit, map[string]string{"hint": params.hint})
	if err != nil {
		logger.Error("Failed to get networks", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Serialize the networks to JSON
	jsonResponse, err := json.Marshal(networks)
	if err != nil {
		logger.Error("Failed to marshal networks into JSON", zap.Error(err))
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

func (s *server) GetNetworkHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.NetworkService)
}
