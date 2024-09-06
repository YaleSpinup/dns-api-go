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
	UpdateMacAddress(newMac models.Mac) error
}

type MacAddressService struct {
	server interfaces.ServerInterface
	interfaces.EntityGetter
	interfaces.EntityUpdater
}

// NewMacAddressService Constructor for MacAddressService
func NewMacAddressService(server interfaces.ServerInterface, entityGetter interfaces.EntityGetter, entityUpater interfaces.EntityUpdater) *MacAddressService {
	return &MacAddressService{server: server, EntityGetter: entityGetter, EntityUpdater: entityUpater}
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
	var bluecatEntity models.BluecatEntity
	if err := json.Unmarshal(resp, &bluecatEntity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}

	// Check if the response represents an empty entity
	if bluecatEntity.IsEmpty() {
		logger.Info("Entity not found", zap.String("macAddress", macAddress))
		return nil, &ErrEntityNotFound{}
	}

	// Convert BluecatEntity to Entity
	entity := bluecatEntity.ToEntity()

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

	// Associate mac address with a pool if PoolId exists
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

func (ms *MacAddressService) UpdateMacAddress(newMac models.Mac) error {
	logger.Info("UpdateMacAddress started", zap.Any("New MAC", newMac))

	// Associate mac address with a pool if poolid exists
	if newMac.PoolId != 0 {
		// Get the configuration ID
		configId, err := GetConfigID(ms.server)
		if err != nil {
			return err
		}

		if err := ms.AssociateMacAddress(newMac, configId); err != nil {
			return err
		}
	}

	// Return early if newProperties is empty
	if len(newMac.Properties) == 0 {
		logger.Info("No new properties to update")
		return nil
	}

	// Get the mac object from inside bluecat to retrieve the properties
	entity, err := ms.GetMacAddress(newMac.Address)
	if err != nil {
		return err
	}

	// Merge new properties into existing entity properties
	for key, value := range newMac.Properties {
		entity.Properties[key] = value
	}

	// Update entity in bluecat
	err = ms.EntityUpdater.UpdateEntity(entity)
	if err != nil {
		return err
	}

	logger.Info("UpdateMacAddress successful", zap.String("macAddress", newMac.Address))
	return nil
}
