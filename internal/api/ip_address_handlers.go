package api

import (
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"net"
	"net/http"
)

type IpAddressParams struct {
	Address string
}

type AssignIpAddressParams struct {
	MacAddress  string            `json:"mac"`
	ParentId    int               `json:"network_id"`
	Hostname    string            `json:"hostname"`
	ReverseFlag string            `json:"reverse"`
	CIDR        string            `json:"cidr"`
	Properties  map[string]string `json:"properties"`
}

// parseIpAddressParams parses and validates the parameters from the request.
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

// parseAssignIpAddressParams parses and validates the parameters from the request.
func parseAssignIpAddressBody(s *server, ipAddressService services.IpAddressEntityService, r *http.Request) (*AssignIpAddressParams, error) {
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

	// If there is no parent id provided, attempt to find it from the provided CIDR
	if AssignIpAddressParams.ParentId == 0 {
		cidrParentId, err := parentIdFromCidr(s, ipAddressService, AssignIpAddressParams.CIDR)
		if err != nil {
			return nil, fmt.Errorf("you must either pass a valid network_id or a valid CIDR")
		}

		AssignIpAddressParams.ParentId = cidrParentId
	}

	// Validate the presence of hostname
	if AssignIpAddressParams.Hostname == "" {
		return nil, fmt.Errorf("missing required parameter: hostname")
	}

	// Validate the presence of reverse flag
	if AssignIpAddressParams.ReverseFlag == "" {
		return nil, fmt.Errorf("missing required parameter: reverse")
	}
	// ReverseFlag must be true or false
	if AssignIpAddressParams.ReverseFlag != "true" && AssignIpAddressParams.ReverseFlag != "false" {
		return nil, fmt.Errorf("reverse flag must be true or false")
	}

	// Initialize Properties if nil
	if AssignIpAddressParams.Properties == nil {
		AssignIpAddressParams.Properties = make(map[string]string)
	}

	return &AssignIpAddressParams, nil
}

// parentIdFromCidr returns the parent ID for the given CIDR range.
func parentIdFromCidr(s *server, ipAddressService services.IpAddressEntityService, cidr string) (int, error) {
	// Check if cidr is empty
	if cidr == "" {
		return -1, fmt.Errorf("CIDR cannot be empty")
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return -1, fmt.Errorf("invalid CIDR format: %v", err)
	}

	// Enumerate the first 10 IP addresses in the CIDR range
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		logger.Info("Trying to use IP address as canary", zap.String("ip", ip.String()))

		// Get the IP address entity from the database
		ipAddressEntity, err := ipAddressService.GetIpAddress(ip.String())
		if err != nil {
			continue
		}

		// Attempt to find parent ID of the IP address entity
		parentID, err := services.GetParentID(s, ipAddressEntity.ID)
		if err == nil {
			return parentID, nil
		}
	}

	// No parent ID found in the CIDR range
	return 0, fmt.Errorf("no valid parent ID found in the CIDR range")
}

// incrementIP increments the given IP address by 1.
func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
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

// DeleteIpAddressHandler deletes an ip address entity from the bluecat
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

// AssignIpAddressHandler assigns the next available ipv4 address to a host in bluecat
func (s *server) AssignIpAddressHandler(w http.ResponseWriter, r *http.Request) {
	// Parse the body from the request
	body, err := parseAssignIpAddressBody(s, s.services.IpAddressService, r)
	if err != nil {
		logger.Warn("Invalid request body", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Set hostInfo
	hostInfo := map[string]string{
		"hostname":       body.Hostname,
		"viewId":         s.bluecat.viewId,
		"reverseFlag":    body.ReverseFlag,
		"sameAsZoneFlag": "false",
	}

	// Add name property to the properties map
	body.Properties["name"] = body.Hostname

	// Assign the ip address and handle potential errors
	entity, err := s.services.IpAddressService.AssignIpAddress(
		"MAKE_STATIC", body.MacAddress, body.ParentId, hostInfo, body.Properties)
	if err != nil {
		logger.Error("Error assigning ip address", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Create temporary struct that adds ip field to entity fields
	type tempEntity struct {
		ID         int               `json:"id"`
		Name       string            `json:"name"`
		Type       string            `json:"type"`
		Properties map[string]string `json:"properties"`
		IP         string            `json:"ip"`
	}

	// Populate the temporary struct
	entityWithIP := tempEntity{
		ID:         entity.ID,
		Name:       entity.Name,
		Type:       entity.Type,
		Properties: entity.Properties,
		IP:         entity.Properties["address"],
	}

	// Successfully assigned ip address; sending back to client
	s.respond(w, entityWithIP, http.StatusOK)
}

// GetCIDRHandler retrieves the CIDR file from the server
func (s *server) GetCIDRHandler(w http.ResponseWriter, _ *http.Request) {
	contents, err := s.GetCIDRFile()
	if err != nil {
		logger.Error("Error getting CIDR file", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.respond(w, contents, http.StatusOK)
}
