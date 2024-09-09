package api

import (
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net/http"
)

type IpAddressParams struct {
	Address string
}

type AssignIpAddressParams struct {
	MacAddress  string `json:"mac"`
	ParentId    int    `json:"network_id"`
	Hostname    string `json:"hostname"`
	ReverseFlag bool   `json:"reverse"`
}

func parseIpAddressParams(r *http.Request) (*IpAddressParams, error) {
	// Extract address parameter from the request URL
	vars := mux.Vars(r)
	address, addressOk := vars["address"]

	// Validate the presence of the required 'address' parameter
	if !addressOk {
		return nil, fmt.Errorf("missing required parameter: id")
	}

	return &IpAddressParams{
		Address: address,
	}, nil
}

func parseAssignIpAddressParams(r *http.Request) (*AssignIpAddressParams, error) {
	var AssignIpAddressParams AssignIpAddressParams

	// Extract the parameters from the request body
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&AssignIpAddressParams); err != nil {
		return nil, fmt.Errorf("failed to decode request body: %v", err)
	}

	// Validate the presence and format of the mac address
	if err := validateMacAddress(AssignIpAddressParams.MacAddress); err != nil {
		return nil, err
	}

	// TODO: Validate the presence of network_id

	// Validate the presence of hostname
	if AssignIpAddressParams.Hostname == "" {
		return nil, fmt.Errorf("missing required parameter: hostname")
	}

	// Validate the presence of reverse flag
	if AssignIpAddressParams.ReverseFlag == false {
		return nil, fmt.Errorf("missing required parameter: reverse")
	}

	return &AssignIpAddressParams, nil
}

// GetIpAddressHandler retrieves an ip address entity from the database
func (s *server) GetIpAddressHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the ip address parameter from the request
	params, err := parseIpAddressParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Attempt to retrieve the ip address entity and handle potential errors
	entity, err := s.services.IpAddressService.GetIpAddress(params.Address)
	if err != nil {
		logger.Error("Error retrieving ip address entity",
			zap.String("address", params.Address),
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

func (s *server) DeleteIpAddressHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the entity parameters from the request
	params, err := parseIpAddressParams(r)
	if err != nil {
		logger.Warn("Invalid request parameters", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Attempt to delete the ip address and handle potential errors
	err = s.services.IpAddressService.DeleteIpAddress(params.Address)
	if err != nil {
		logger.Error("Error deleting ip address", zap.String("address", params.Address), zap.Error(err))

		// Determine the type of error and set the HTTP response accordingly
		switch e := err.(type) {
		case *services.ErrEntityNotFound:
			http.Error(w, e.Error(), http.StatusNotFound)
			return
		case *services.ErrDeleteNotAllowed:
			http.Error(w, e.Error(), http.StatusForbidden)
			return
		case *services.ErrEntityTypeMismatch:
			http.Error(w, e.Error(), http.StatusConflict)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
func (s *server) AssignIpAddressHandler(w http.ResponseWriter, r *http.Request) {

}
