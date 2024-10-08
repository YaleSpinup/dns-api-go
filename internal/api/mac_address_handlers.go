package api

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/models"
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
)

type MacAddressParams struct {
	Address string
}

type MacParams struct {
	Address    string `json:"mac"`
	PoolId     int    `json:"macpool"`
	Properties string `json:"properties"`
}

// parseMacAddressParams parses and validates the parameters from the request.
func parseMacAddressParams(r *http.Request) (*MacAddressParams, error) {
	// Extract mac address parameter from the request URL
	vars := mux.Vars(r)
	macAddress, macAddressOk := vars["mac"]

	// Validate the presence of the required 'mac' parameter
	if !macAddressOk {
		return nil, fmt.Errorf("missing required parameter: mac")
	}
	// Make sure mac address is in the correct format
	if err := validateMacAddress(macAddress); err != nil {
		return nil, err
	}

	return &MacAddressParams{Address: macAddress}, nil
}

// parseCreateMacParams parses and validates the parameters from the request.
func parseCreateMacParams(r *http.Request) (*MacParams, error) {
	var MacParams MacParams

	// Extract the mac parameters from the request body
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&MacParams); err != nil {
		return nil, fmt.Errorf("failed to decode request body: %v", err)
	}

	// Validate the presence and format of the mac address
	if err := validateMacAddress(MacParams.Address); err != nil {
		return nil, err
	}

	return &MacParams, nil
}

// parseMacParams parses and validates the parameters from the request.
func parseUpdateMacParams(r *http.Request) (*MacParams, error) {
	var MacParams MacParams
	// Extract the mac address parameter from URL
	vars := mux.Vars(r)
	macAddress, macAddressOk := vars["mac"]

	// Validate the presence of the required 'mac' parameter
	if !macAddressOk {
		return nil, fmt.Errorf("missing required parameter: mac")
	}
	// Make sure mac address is in the correct format
	if err := validateMacAddress(macAddress); err != nil {
		return nil, err
	}

	// Set the extracted mac address
	MacParams.Address = macAddress

	// Extract the rest of the parameters from the request body
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&MacParams); err != nil {
		return nil, fmt.Errorf("failed to decode request body: %v", err)
	}

	return &MacParams, nil
}

// GetMacAddressHandler handles GET requests for retrieving a mac address by address.
func (s *server) GetMacAddressHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the mac parameters from the request
	params, err := parseMacAddressParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Attempt to get the mac address entity and handle potential errors
	entity, err := s.services.MacAddressService.GetMacAddress(params.Address)
	if err != nil {
		logger.Error("Error getting mac address entity", zap.String("macAddress", params.Address), zap.Error(err))
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

// CreateMacAddressHandler handles POST requests for creating a mac address.
func (s *server) CreateMacAddressHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the mac parameters from the request
	params, err := parseCreateMacParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert properties into a map
	propertiesMap := common.ConvertToMap(params.Properties, "|")

	// Create mac object
	mac := &models.Mac{
		Address:    params.Address,
		PoolId:     params.PoolId,
		Properties: propertiesMap,
	}

	// Attempt to create the mac address to bluecat and handle potential errors
	objectId, err := s.services.MacAddressService.CreateMacAddress(*mac)
	if err != nil {
		logger.Error("Failed to create mac address", zap.Error(err))
		switch e := err.(type) {
		case *services.ErrEntityAlreadyExists:
			http.Error(w, e.Error(), http.StatusConflict)
		case *services.PoolIDError:
			http.Error(w, e.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Send the response back to client with objectId of newly created Mac object
	s.respond(w, objectId, http.StatusOK)
}

// UpdateMacAddressHandler handles PUT requests for updating a mac address.
func (s *server) UpdateMacAddressHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the mac parameters from the request
	params, err := parseUpdateMacParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert properties into a map
	propertiesMap := common.ConvertToMap(params.Properties, "|")

	// Create new mac object with new properties
	mac := &models.Mac{
		Address:    params.Address,
		PoolId:     params.PoolId,
		Properties: propertiesMap,
	}

	// Update the mac object with the new properties
	err = s.services.MacAddressService.UpdateMacAddress(*mac)
	if err != nil {
		logger.Error("Failed to update mac address", zap.Error(err))
		// Determine the type of error and set the HTTP response accordingly
		switch e := err.(type) {
		case *services.ErrEntityNotFound:
			http.Error(w, e.Error(), http.StatusNotFound)
			return
		case *services.PoolIDError:
			http.Error(w, e.Error(), http.StatusBadRequest)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Send the response back to client
	s.respond(w, nil, http.StatusNoContent)
}
