package api

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/services"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type IpAddressParams struct {
	Address string
}

type AssignIpAddressParams struct {
	MacAddress  string `json:"mac" example:"00:11:22:33:44:55"`
	ParentId    int    `json:"network_id" example:"12345"`
	Hostname    string `json:"hostname" example:"server-001"`
	ReverseFlag bool   `json:"reverse" example:"true"`
	CIDR        string `json:"cidr" example:"10.0.1.0/24"`
	Properties  string `json:"properties" example:"department=IT|environment=prod"`
}

// parseIpAddressParams parses and validates the parameters from the request.
func parseIpAddressParams(r *http.Request) (*IpAddressParams, error) {
	// Extract address parameter from the request URL
	vars := mux.Vars(r)
	address, addressOk := vars["ip"]

	// Validate the presence of the required 'address' parameter
	if !addressOk {
		return nil, fmt.Errorf("missing required parameter: address")
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
	if !AssignIpAddressParams.ReverseFlag {
		return nil, fmt.Errorf("missing required parameter: reverse")
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

	counter := 0
	// Enumerate the first 10 IP addresses in the CIDR range
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		// Only try a max of 10 addresses
		if counter >= 10 {
			break
		}

		logger.Info("Trying to use IP address as canary", zap.String("ip", ip.String()))

		// Get the IP address entity from the database
		ipAddressEntity, err := ipAddressService.GetIpAddress(ip.String())
		if err != nil {
			counter++
			continue
		}

		// Attempt to find parent ID of the IP address entity
		parentID, err := services.GetParentID(s, ipAddressEntity.ID)
		if err == nil {
			return parentID, nil
		}

		counter++
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

// GetIpAddressHandler retrieves an IP address entity from BlueCat
// @Summary Get IP address details
// @Description Retrieves detailed information about a specific IP address including its properties and allocation status
// @Tags IP Address Management
// @Produce json
// @Param account path string true "Account identifier"
// @Param ip path string true "IP address" format(ipv4)
// @Success 200 {object} models.Entity "IP address details"
// @Failure 400 "Invalid request parameters"
// @Failure 404 "IP address not found"
// @Failure 500 "Internal server error"
// @Router /{account}/ips/{ip} [get]
func (s *server) GetIpAddressHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("GetIpAddressHandler started")

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

// DeleteIpAddressHandler deletes an IP address entity from BlueCat
// @Summary Delete IP address
// @Description Permanently deletes an IP address allocation from BlueCat, freeing it for reuse
// @Tags IP Address Management
// @Param account path string true "Account identifier"
// @Param ip path string true "IP address to delete" format(ipv4)
// @Success 204 "IP address successfully deleted"
// @Failure 400 "Invalid request parameters"
// @Failure 403 "Delete operation not allowed"
// @Failure 404 "IP address not found"
// @Failure 409 "Entity type mismatch"
// @Failure 500 "Internal server error"
// @Router /{account}/ips/{ip} [delete]
func (s *server) DeleteIpAddressHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("DeleteIpAddressHandler started")

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

	// Successfully deleted ip address; sending back to client
	logger.Info("DeleteIpAddressHandler completed")
	s.respond(w, nil, http.StatusNoContent)

}

// AssignIpAddressHandler assigns the next available IPv4 address to a host in BlueCat
// @Summary Assign next available IP address
// @Description Assigns the next available IPv4 address in the specified network to a host with the given MAC address and hostname. Creates associated DNS records if requested.
// @Tags IP Address Management
// @Accept json
// @Produce json
// @Param account path string true "Account identifier"
// @Param request body AssignIpAddressParams true "IP assignment parameters"
// @Success 200 {object} map[string]interface{} "Successfully assigned IP address with details"
// @Failure 400 "Invalid request parameters or malformed request body"
// @Failure 500 "Internal server error or IP assignment failed"
// @Router /{account}/ips [post]
func (s *server) AssignIpAddressHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("AssignIpAddressHandler started")

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
		"reverseFlag":    fmt.Sprintf("%t", body.ReverseFlag),
		"sameAsZoneFlag": "false",
	}

	// Convert properties into a map and add "name" property
	propertiesMap := common.ConvertToMap(body.Properties, "|")
	propertiesMap["name"] = body.Hostname

	// Assign the ip address and handle potential errors
	entity, err := s.services.IpAddressService.AssignIpAddress(
		"MAKE_STATIC", body.MacAddress, body.ParentId, hostInfo, propertiesMap)
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

// GetCIDRHandler retrieves the CIDR configuration file from the server
// @Summary Get CIDR configuration
// @Description Retrieves the CIDR configuration file containing network ranges and allocation information
// @Tags IP Address Management
// @Produce json
// @Param account path string true "Account identifier"
// @Success 200 {object} map[string]interface{} "CIDR configuration data"
// @Failure 500 "Internal server error or CIDR file not found"
// @Router /{account}/ips/cidrs [get]
func (s *server) GetCIDRHandler(w http.ResponseWriter, _ *http.Request) {
	logger.Info("GetCIDRHandler started")

	contents, err := s.GetCIDRFile()
	if err != nil {
		logger.Error("Error getting CIDR file", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.respond(w, contents, http.StatusOK)
}
