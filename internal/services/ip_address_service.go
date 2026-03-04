package services

import (
	"dns-api-go/internal/bluecat"
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/url"
	"strings"
)

type IpAddressEntityService interface {
	GetIpAddress(address string) (*models.Entity, error)
	DeleteIpAddress(address string) error
	AssignIpAddress(action string, macAddress string, parentId int, hostInfo map[string]string, properties map[string]string) (*models.Entity, error)
}

type IpAddressService struct {
	server interfaces.ServerInterface
}

// NewIpAddressService creates a new IpAddressService
func NewIpAddressService(server interfaces.ServerInterface) *IpAddressService {
	return &IpAddressService{server: server}
}

// GetIpAddress gets an ip entity from bluecat based on the ip address
func (ips *IpAddressService) GetIpAddress(address string) (*models.Entity, error) {
	logger.Info("GetIpAddress started", zap.String("address", address))

	// V2: GET /api/v2/ipv4Addresses?filter=address:eq('{addr}')
	route := "/api/v2/ipv4Addresses"
	params := fmt.Sprintf("filter=%s&limit=1", url.QueryEscape(fmt.Sprintf("address:eq('%s')", address)))
	resp, err := ips.server.MakeRequest("GET", route, params, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		logger.Info("Entity not found", zap.String("ip address", address))
		return nil, &ErrEntityNotFound{}
	}
	logger.Info("Received response for GetIpAddress", zap.ByteString("response", resp))

	// Unmarshal V2 collection response
	var collection bluecat.V2Collection
	if err := json.Unmarshal(resp, &collection); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}

	if len(collection.Data) == 0 {
		logger.Info("Entity not found", zap.String("ip address", address))
		return nil, &ErrEntityNotFound{}
	}

	entity := collection.Data[0].ToEntity()

	logger.Info("GetIpAddress successful", zap.String("ip address", address))
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
	err = DeleteEntityByID(ips.server, entity.ID, []string{types.IP4ADDRESS})
	if err != nil {
		return err
	}

	logger.Info("DeleteIpAddress successfull", zap.String("address", address))
	return nil
}

// AssignIpAddress assigns the next available ipv4 address to a mac address in bluecat
func (ips *IpAddressService) AssignIpAddress(action string, macAddress string, parentId int, hostInfo map[string]string, properties map[string]string) (*models.Entity, error) {
	logger.Info("AssignIpAddress started", zap.String("action", action), zap.String("mac address", macAddress))

	// V2: POST /api/v2/ipv4Addresses with JSON body for next-available assignment
	route := "/api/v2/ipv4Addresses"
	v2Body := map[string]interface{}{
		"action":      action,
		"parentId":    parentId,
		"macAddress":  macAddress,
		"properties":  properties,
	}

	// Add host info fields
	if hostname, ok := hostInfo["hostname"]; ok && hostname != "" {
		v2Body["hostname"] = hostname
	}
	if viewId, ok := hostInfo["viewId"]; ok && viewId != "" {
		v2Body["viewId"] = viewId
	}
	if reverseFlag, ok := hostInfo["reverseFlag"]; ok {
		v2Body["reverseFlag"] = reverseFlag
	}
	if sameAsZoneFlag, ok := hostInfo["sameAsZoneFlag"]; ok {
		v2Body["sameAsZoneFlag"] = sameAsZoneFlag
	}

	bodyJSON, err := json.Marshal(v2Body)
	if err != nil {
		logger.Error("Error marshalling assign IP request", zap.Error(err))
		return nil, err
	}

	body := strings.NewReader(string(bodyJSON))
	resp, err := ips.server.MakeRequest("POST", route, "", body)
	if err != nil {
		return nil, err
	}
	logger.Info("Received response for AssignIpAddress", zap.ByteString("response", resp))

	// Unmarshal V2 entity response
	var v2Entity bluecat.V2Entity
	if err := json.Unmarshal(resp, &v2Entity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}

	entity := v2Entity.ToEntity()

	logger.Info("AssignIpAddress successful", zap.Int("entity id", entity.ID))
	return &entity, nil
}
