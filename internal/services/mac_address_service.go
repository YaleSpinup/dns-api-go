package services

import (
	"dns-api-go/internal/bluecat"
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/url"
	"strings"
)

type MacAddressEntityService interface {
	GetMacAddress(macAddress string) (*models.Entity, error)
	CreateMacAddress(mac models.Mac) (int, error)
	UpdateMacAddress(newMac models.Mac) error
}

type MacAddressService struct {
	server interfaces.ServerInterface
}

// NewMacAddressService Constructor for MacAddressService
func NewMacAddressService(server interfaces.ServerInterface) *MacAddressService {
	return &MacAddressService{server: server}
}

// GetMacAddress Retrieves a mac address entity from bluecat
func (ms *MacAddressService) GetMacAddress(macAddress string) (*models.Entity, error) {
	logger.Info("GetMacAddress started", zap.String("macAddress", macAddress))

	// V2: GET /api/v2/macAddresses?filter=address:eq('{mac}')
	route := "/api/v2/macAddresses"
	params := fmt.Sprintf("filter=%s&limit=1", url.QueryEscape(fmt.Sprintf("address:eq('%s')", macAddress)))
	resp, err := ms.server.MakeRequest("GET", route, params, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		logger.Info("Entity not found", zap.String("macAddress", macAddress))
		return nil, &ErrEntityNotFound{}
	}
	logger.Info("Received response for GetMacAddress", zap.ByteString("response", resp))

	// Unmarshal V2 collection response
	var collection bluecat.V2Collection
	if err := json.Unmarshal(resp, &collection); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}

	if len(collection.Data) == 0 {
		logger.Info("Entity not found", zap.String("macAddress", macAddress))
		return nil, &ErrEntityNotFound{}
	}

	entity := collection.Data[0].ToEntity()

	logger.Info("GetMacAddress successful", zap.String("macAddress", macAddress))
	return &entity, nil
}

// CreateMacAddress Creates a mac address entity in bluecat
func (ms *MacAddressService) CreateMacAddress(mac models.Mac) (int, error) {
	logger.Info("CreateMacAddress started", zap.Any("mac", mac))

	// Check if the mac address entity already exists
	_, err := ms.GetMacAddress(mac.Address)
	if err == nil {
		// Entity already exists, return custom error
		logger.Error("MAC address entity already exists", zap.String("macAddress", mac.Address))
		return -1, &ErrEntityAlreadyExists{EntityID: mac.Address}
	}

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

	// V2: POST /api/v2/macAddresses with JSON body
	route := "/api/v2/macAddresses"
	v2Body := map[string]interface{}{
		"address":    mac.Address,
		"properties": mac.Properties,
	}

	bodyJSON, err := json.Marshal(v2Body)
	if err != nil {
		logger.Error("Error marshalling mac address request", zap.Error(err))
		return -1, err
	}

	body := strings.NewReader(string(bodyJSON))
	resp, err := ms.server.MakeRequest("POST", route, "", body)
	if err != nil {
		return -1, err
	}
	logger.Info("Received response for AddMacAddress", zap.ByteString("response", resp))

	// Unmarshal V2 entity response to get the ID
	var v2Entity bluecat.V2Entity
	if err := json.Unmarshal(resp, &v2Entity); err != nil {
		logger.Error("Error unmarshalling mac address response", zap.Error(err))
		return -1, err
	}

	return v2Entity.ID, nil
}

// AssociateMacAddress Associates a MAC address with a MAC pool in bluecat
func (ms *MacAddressService) AssociateMacAddress(mac models.Mac, configId int) error {
	logger.Info("AssociateMacAddress started", zap.String("macAddress", mac.Address), zap.Int("poolId", mac.PoolId))

	// V2: PUT /api/v2/macAddresses with pool association
	// First get the MAC address entity to get its ID
	entity, err := ms.GetMacAddress(mac.Address)
	if err != nil {
		return &PoolIDError{PoolID: mac.PoolId, Err: err}
	}

	// V2: PUT /api/v2/macAddresses/{id} with macPool link
	route := fmt.Sprintf("/api/v2/macAddresses/%d", entity.ID)
	v2Body := map[string]interface{}{
		"macPool": map[string]interface{}{
			"id": mac.PoolId,
		},
	}

	bodyJSON, err := json.Marshal(v2Body)
	if err != nil {
		return &PoolIDError{PoolID: mac.PoolId, Err: err}
	}

	body := strings.NewReader(string(bodyJSON))
	_, err = ms.server.MakeRequest("PUT", route, "", body)
	if err != nil {
		return &PoolIDError{PoolID: mac.PoolId, Err: err}
	}

	logger.Info("AssociateMacAddress successful", zap.String("macAddress", mac.Address), zap.Int("poolId", mac.PoolId))
	return nil
}

func (ms *MacAddressService) UpdateMacAddress(newMac models.Mac) error {
	logger.Info("UpdateMacAddress started", zap.Any("New MAC", newMac))

	// Check if mac object exists and if it does, the properties
	entity, err := ms.GetMacAddress(newMac.Address)
	if err != nil {
		return err
	}

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

	// Merge new properties into existing entity properties
	for key, value := range newMac.Properties {
		entity.Properties[key] = value
	}

	// Update entity in bluecat
	err = UpdateEntity(ms.server, entity)
	if err != nil {
		return err
	}

	logger.Info("UpdateMacAddress successful", zap.String("macAddress", newMac.Address))
	return nil
}
