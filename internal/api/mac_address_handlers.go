package api

import (
	"dns-api-go/internal/models"
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
	"regexp"
)

type MacAddressParams struct {
	Address string
}

type MacParams struct {
	Address    string            `json:"mac"`
	PoolId     int               `json:"macpool"`
	Properties map[string]string `json:"properties"`
}

// validateMacAddress validates the format of the MAC address
// mac address should be in the format: nnnnnnnnnnnn or nn:nn:nn:nn:nn:nn or nn-nn-nn-nn-nn-nn
func validateMacAddress(macAddress string) error {
	// Define the regular expression for a valid MAC address
	macRegex := `^([0-9A-Fa-f]{12}|([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2})$`
	re := regexp.MustCompile(macRegex)

	// Validate the MAC address format
	if !re.MatchString(macAddress) {
		return fmt.Errorf("invalid MAC address format. MAC address should be in the format: nnnnnnnnnnnn or nn:nn:nn:nn:nn:nn or nn-nn-nn-nn-nn-nn")
	}

	return nil
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
	if MacParams.Address == "" {
		return nil, fmt.Errorf("missing required parameter: mac")
	}
	if err := validateMacAddress(MacParams.Address); err != nil {
		return nil, err
	}

	// Initialize Properties if nil
	if MacParams.Properties == nil {
		MacParams.Properties = make(map[string]string)
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

	// Initialize Properties if nil
	if MacParams.Properties == nil {
		MacParams.Properties = make(map[string]string)
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

	// Create mac object
	mac := &models.Mac{
		Address:    params.Address,
		PoolId:     params.PoolId,
		Properties: params.Properties,
	}

	// Check if the mac address entity already exists
	_, err = s.services.MacAddressService.GetMacAddress(mac.Address)
	if err == nil {
		// Entity already exists, return custom error
		logger.Error("MAC address entity already exists", zap.String("macAddress", mac.Address))
		existsErr := &services.ErrEntityAlreadyExists{EntityID: mac.Address}
		http.Error(w, existsErr.Error(), http.StatusConflict)
		return
	}

	// Attempt to create the mac address to bluecat and handle potential errors
	objectId, err := s.services.MacAddressService.CreateMacAddress(*mac)
	if err != nil {
		logger.Error("Failed to create mac address", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	// Create new mac object with new properties
	mac := &models.Mac{
		Address:    params.Address,
		PoolId:     params.PoolId,
		Properties: params.Properties,
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
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Send the response back to client
	s.respond(w, nil, http.StatusNoContent)
}
