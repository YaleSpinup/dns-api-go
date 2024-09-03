package services

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
)

type MacAddressEntityService interface {
	GetMacAddress(macAddress string) (*models.Entity, error)
	CreateMacAddress(mac models.Mac) (int, error)
	UpdateMacAddress(mac models.Mac, properties map[string]string) error
}

type MacAddressService struct {
	server interfaces.ServerInterface
	interfaces.EntityGetter
}

// NewMacAddressService Constructor for MacAddressService
func NewMacAddressService(server interfaces.ServerInterface, entityGetter interfaces.EntityGetter) *MacAddressService {
	return &MacAddressService{server: server, EntityGetter: entityGetter}
}

// GetMacAddress Retrieves a mac address entity from bluecat
func (ms *MacAddressService) GetMacAddress(macAddress string) (*models.Entity, error) {
	logger.Info("GetMacAddress started", zap.String("macAddress", macAddress))

	// Get the configuration ID
	configId, err := GetConfigID(ms.server)
	if err != nil {
		return nil, err
	}

	// Send http request to bluecat
	route, params := "/getMACAddress", fmt.Sprintf("configurationId=%d&macAddress=%s", configId, macAddress)
	resp, err := ms.server.MakeRequest("GET", route, params)
	if err != nil {
		return nil, err
	}
	logger.Info("Received response for GetMacAddress", zap.ByteString("response", resp))

	// Unmarshal the response
	var entityResp models.EntityResponse
	if err := json.Unmarshal(resp, &entityResp); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}

	// Check if the response represents an empty entity
	if entityResp.IsEmpty() {
		logger.Info("Entity not found", zap.String("macAddress", macAddress))
		return nil, &ErrEntityNotFound{}
	}

	// Convert EntityResponse to Entity
	entity := entityResp.ToEntity()

	logger.Info("GetMacAddress successful", zap.String("macAddress", macAddress))
	return &entity, nil
}

// CreateMacAddress Creates a mac address entity in bluecat
func (ms *MacAddressService) CreateMacAddress(mac models.Mac) (int, error) {
	logger.Info("CreateMacAddress started", zap.Any("mac", mac))

	// Get the configuration ID
	configId, err := GetConfigID(ms.server)
	if err != nil {
		return -1, err
	}

	// Add mac address to bluecat
	objectId, err := ms.AddMacAddress(mac, configId)
	if err != nil {
		return -1, err
	}

	// Associate mac address with a pool if poolid exists
	if mac.PoolId != 0 {
		if err := ms.AssociateMacAddress(mac, configId); err != nil {
			return -1, err
		}
	}

	return objectId, nil
}

// AddMacAddress Adds a mac address entity in bluecat
func (ms *MacAddressService) AddMacAddress(mac models.Mac, configId int) (int, error) {
	logger.Info("AddMacAddress started", zap.Any("mac", mac))

	// Send request to bluecat
	route, params := "/addMACAddress", fmt.Sprintf("configurationId=%d&macAddress=%s",
		configId, mac.Address)
	params += "&properties=" + common.ConvertToSeparatedString(mac.Properties, "|")
	resp, err := ms.server.MakeRequest("POST", route, params)
	if err != nil {
		return -1, err
	}
	logger.Info("Received response for AddMacAddress", zap.ByteString("response", resp))

	// Unmarshal the response to get the object iD
	var objectId int
	if err := json.Unmarshal(resp, &objectId); err != nil {
		logger.Error("Error unmarshalling objectId", zap.Error(err))
		return -1, err
	}

	return objectId, nil
}

// AssociateMacAddress Associates a MAC address with a MAC pool in bluecat
func (ms *MacAddressService) AssociateMacAddress(mac models.Mac, configId int) error {
	logger.Info("AssociateMacAddress started", zap.String("macAddress", mac.Address), zap.Int("poolId", mac.PoolId))

	// Send request to bluecat
	route, params := "/associateMACAddressWithPool", fmt.Sprintf("configurationId=%d&macAddress=%s&poolId=%d",
		configId, mac.Address, mac.PoolId)
	resp, err := ms.server.MakeRequest("POST", route, params)
	if err != nil {
		return err
	}

	logger.Info("Received response for AssociateMacAddress", zap.ByteString("response", resp))
	return nil
}

func (ms *MacAddressService) UpdateMacAddress() {

}
