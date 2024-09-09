package services

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
)

type IpAddressEntityService interface {
	GetIpAddress(address string) (*models.Entity, error)
	DeleteIpAddress(address string) error
	AssignIpAddress(action string, macAddress string, parentId int, hostInfo map[string]string, properties map[string]string) (*models.Entity, error)
}

type IpAddressService struct {
	server        interfaces.ServerInterface
	EntityDeleter interfaces.EntityDeleter
}

// NewIpAddressService creates a new IpAddressService
func NewIpAddressService(server interfaces.ServerInterface, entityDeleter interfaces.EntityDeleter) *IpAddressService {
	return &IpAddressService{server: server, EntityDeleter: entityDeleter}
}

// GetIpAddress gets an ip entity from bluecat based on the ip address
func (ips *IpAddressService) GetIpAddress(address string) (*models.Entity, error) {
	logger.Info("GetIpAddress started", zap.String("address", address))

	// Get the container ID
	containerId, err := GetConfigID(ips.server)
	if err != nil {
		return nil, err
	}

	// Send http request to bluecat
	route, params := "/getIP4Address", fmt.Sprintf("address=%s&containerId=%d", address, containerId)
	resp, err := ips.server.MakeRequest("GET", route, params, nil)
	if err != nil {
		return nil, err
	}
	logger.Info("Received response for GetIpAddress", zap.ByteString("response", resp))

	// Unmarshal the response
	var bluecatEntity models.BluecatEntity
	if err := json.Unmarshal(resp, &bluecatEntity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}

	// Check if the response represents an empty entity
	if bluecatEntity.IsEmpty() {
		logger.Info("Entity not found", zap.String("ip address", address))
		return nil, &ErrEntityNotFound{}
	}

	// Convert BluecatEntity to Entity
	entity := bluecatEntity.ToEntity()

	logger.Info("GetIpAddress successfull", zap.String("ip address", address))
	return &entity, nil
}

// DeleteIpAddress deletes an ip address from bluecat
func (ips *IpAddressService) DeleteIpAddress(address string) error {
	logger.Info("DeleteIpAddress started", zap.String("address", address))

	// Get the id of the ip address
	entity, err := ips.GetIpAddress(address)
	if err != nil {
		return err
	}
	logger.Info("Found ip address", zap.String("address", address), zap.Int("id", entity.ID))

	// Delete the ip address
	err = ips.EntityDeleter.DeleteEntity(entity.ID, []string{types.IP4ADDRESS})
	if err != nil {
		return err
	}

	logger.Info("DeleteIpAddress successfull", zap.String("address", address))
	return nil
}

// AssignIpAddress assigns the next available ipv4 address to a mac address in bluecat
func (ips *IpAddressService) AssignIpAddress(action string, macAddress string, parentId int, hostInfo map[string]string, properties map[string]string) (*models.Entity, error) {
	logger.Info("AssignIpAddress started", zap.String("action", action), zap.String("mac address", macAddress))

	// Get the configuration ID
	configId, err := GetConfigID(ips.server)
	if err != nil {
		return nil, err
	}

	// Create hostInfo string
	hostInfoString := fmt.Sprintf("%s,%s,%s,%s",
		hostInfo["hostname"],
		hostInfo["viewId"],
		hostInfo["reverseFlag"],
		hostInfo["sameAsZoneFlag"])

	// Create properties string
	propertiesString := common.ConvertToSeparatedString(properties, "|")

	// Send http request to bluecat
	route := "/assignNextAvailableIP4Address"
	params := fmt.Sprintf("action=%s&configurationId=%d&hostInfo=%s&macAddress=%s&parentId=%d&properties=%s",
		action, configId, hostInfoString, macAddress, parentId, propertiesString)
	resp, err := ips.server.MakeRequest("POST", route, params, nil)
	if err != nil {
		return nil, err
	}
	logger.Info("Received response for AssignIpAddress", zap.ByteString("response", resp))

	// Unmarshal the response
	var bluecatEntity models.BluecatEntity
	if err := json.Unmarshal(resp, &bluecatEntity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}

	// Convert BluecatEntity to Entity
	entity := bluecatEntity.ToEntity()

	logger.Info("AssignIpAddress successfull", zap.Int("entity id", entity.ID))
	return &entity, nil
}
