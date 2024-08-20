package api

import (
	"fmt"
	"net/http"
	"strconv"
)

type GetZonesParams struct {
	offset int
	limit  int
	hint   string
}

type GetZoneParams struct {
	zoneId int
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
	return nil, nil
}

func (s *server) GetZonesHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the entity parameters from the request
}
