package api

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"strconv"
	"strings"
)

type GetRecordsByTypeParams struct {
	recordType string
	offset     int
	limit      int
	name       string
	keyword    string
	hint       string
}

type CreateRecordParams struct {
	RecordType string `json:"type"`
	RecordName string `json:"record"`
	Target     string `json:"target"`
	Properties string `json:"properties"`
	Ttl        int    `json:"ttl"`
}

func (s *server) GetRecordHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.RecordService)
}

func (s *server) DeleteRecordHandler() http.HandlerFunc {
	return s.HandleDeleteEntityReq(s.services.RecordService)
}

func parseGetRecordsByTypeParams(r *http.Request) (*GetRecordsByTypeParams, error) {
	var Params GetRecordsByTypeParams

	// Set default values
	Params.offset = 0
	Params.limit = 10
	Params.hint = ""
	Params.name = ""
	Params.keyword = ""

	query := r.URL.Query()

	// Parse record type from the request URL
	recordType := query.Get("type")
	if recordType == "" {
		return nil, fmt.Errorf("missing required parameter: type")
	}
	// Make sure record type of either HostRecord, AliasRecord, or ExternalHostRecord
	if !common.Contains([]string{"HostRecord", "AliasRecord", "ExternalHostRecord"}, recordType) {
		return nil, fmt.Errorf("invalid record type")
	}
	Params.recordType = recordType

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

	// Parse hint if it is not empty
	if hintStr := query.Get("hint"); hintStr != "" {
		Params.hint = hintStr
	}

	// Parse name if it is not empty
	if nameStr := query.Get("name"); nameStr != "" {
		Params.name = nameStr
	}

	// Parse keyword if it is not empty
	if keywordStr := query.Get("keyword"); keywordStr != "" {
		Params.keyword = keywordStr
	}

	return &Params, nil
}

func parseCreateRecordParams(r *http.Request) (*CreateRecordParams, error) {
	var Params CreateRecordParams

	// Extract the parameters from the request body
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&Params); err != nil {
		return nil, fmt.Errorf("failed to decode request body: %v", err)
	}

	// Validate the presence of the required 'type' parameter
	if Params.RecordType == "" {
		return nil, fmt.Errorf("missing required parameter: type")
	}
	// Make sure record type of either HostRecord, AliasRecord, or ExternalHostRecord
	if common.Contains([]string{"HostRecord", "AliasRecord", "ExternalHostRecord"}, Params.RecordType) == false {
		return nil, fmt.Errorf("invalid record type")
	}

	// Validate the presence of the required 'name' parameter
	if Params.RecordName == "" {
		return nil, fmt.Errorf("missing required parameter: record")
	}

	// If ttl is not specified, set it to 300
	if Params.Ttl == 0 {
		Params.Ttl = 300
	}

	return &Params, nil
}

func (s *server) GetRecordsHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetRecordsHandler started")

	// Parse parameters from the request
	params, err := parseGetRecordsByTypeParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create a map of parameters to pass to the service
	paramMap := map[string]interface{}{
		"start":   params.offset,
		"count":   params.limit,
		"options": map[string]string{"hint": params.hint},
		"name":    params.name,
		"keyword": params.keyword,
	}

	// Get the view id
	viewIdStr := s.bluecat.viewId
	viewId, err := strconv.Atoi(viewIdStr)
	if err != nil {
		logger.Error("Error converting viewId to int", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entities, err := s.services.RecordService.GetRecordsByType(params.recordType, paramMap, viewId)
	if err != nil {
		logger.Error("Error getting records", zap.Error(err))
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

	logger.Info("GetRecordsHandler successful")
	s.respond(w, entities, http.StatusOK)
}

func (s *server) CreateRecordHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("CreateRecordHandler started")

	// Parse parameters from the request
	params, err := parseCreateRecordParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Derive addresses from target and convert properties into a map
	addresses := strings.Split(params.Target, ",")
	propertiesMap := common.ConvertToMap(params.Properties, "|")

	// Get the view id
	viewIdStr := s.bluecat.viewId
	viewId, err := strconv.Atoi(viewIdStr)
	if err != nil {
		logger.Error("Error converting viewId to int", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a map of parameters to pass to the service
	paramMap := map[string]interface{}{
		"absoluteName":     params.RecordName,
		"linkedRecordName": params.Target,
		"name":             params.RecordName,
		"addresses":        addresses,
		"properties":       propertiesMap,
		"ttl":              params.Ttl,
	}

	entity, err := s.services.RecordService.CreateRecord(params.RecordType, paramMap, viewId)
	if err != nil {
		logger.Error("Error creating record", zap.Error(err))
		// Determine the type of error and set the HTTP response accordingly
		switch e := err.(type) {
		case *services.ErrEntityAlreadyExists:
			http.Error(w, e.Error(), http.StatusConflict)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	logger.Info("CreateRecordHandler successful")
	s.respond(w, entity, http.StatusCreated)
}
