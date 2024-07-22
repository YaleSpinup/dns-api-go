package services

import (
	"dns-api-go/common"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
)

type Entity struct {
	ID         int
	Name       string
	Type       string
	Properties string
}

type EntityResponse struct {
	ID         int     `json:"id"`
	Name       *string `json:"name"`
	Type       *string `json:"type"`
	Properties *string `json:"properties"`
}

type EntityService interface {
	GetEntityByID(id string, includeHA string) (*Entity, error)
	DeleteEntityByID(id string) error
}

type GenericEntityService struct {
	server common.ServerInterface
}

// NewGenericEntityService Constructor for GenericEntityService
func NewGenericEntityService(server common.ServerInterface) *GenericEntityService {
	return &GenericEntityService{server: server}
}

func (es *GenericEntityService) GetEntityByID(id string, includeHA string) (*Entity, error) {
	logger.Info("GetEntityByID started", zap.String("id", id), zap.String("includeHA", includeHA))

	// Send http request to bluecat
	route, params := "/getEntityById", fmt.Sprintf("id=%s&includeHA=%s", id, includeHA)
	resp, err := es.server.MakeRequest(route, params)

	// Check for errors when sending request
	if err != nil {
		return nil, err
	}
	logger.Info("Received response for GetEntityByID", zap.ByteString("response", resp))

	// Unmarshal the response
	var entityResp EntityResponse
	if err := json.Unmarshal(resp, &entityResp); err != nil {
		logger.Error("Error unmarshaling entity response", zap.Error(err))
		return nil, err
	}
	// Check if the response represents an empty entity
	if entityResp.ID == 0 && entityResp.Name == nil && entityResp.Type == nil && entityResp.Properties == nil {
		logger.Info("Entity not found", zap.String("id", id))
		return nil, &ErrEntityNotFound{}
	}

	// Convert EntityResponse to Entity
	entity := &Entity{
		ID:         entityResp.ID,
		Name:       *entityResp.Name,
		Type:       *entityResp.Type,
		Properties: *entityResp.Properties,
	}

	logger.Info("GetEntityByID successful",
		zap.Int("entityID", entity.ID),
		zap.String("entityType", entity.Type))
	return entity, nil
}

var ALLOWDELETE = []string{
	"HOSTRECORD",
	"EXTERNALHOST",
	"CNAMERECORD",
	"IP4ADDRESS",
	"MACADDRESS",
	"MACPOOL",
}

func (es *GenericEntityService) DeleteEntityByID(id string) error {
	logger.Info("DeleteEntityByID started", zap.String("id", id))

	// Get the entity type
	entity, err := es.GetEntityByID(id, "false")
	if err != nil {
		return err
	}

	// Check if the entity type is allowed to be deleted
	isAllowedToDelete := false
	for _, allowedType := range ALLOWDELETE {
		if entity.Type == allowedType {
			isAllowedToDelete = true
			break
		}
	}

	if !isAllowedToDelete {
		logger.Info("Entity deletion not allowed", zap.String("id", id), zap.String("type", entity.Type))
		return &ErrDeleteNotAllowed{Type: entity.Type}
	}

	// Send http request to bluecat
	route, params := "/delete", fmt.Sprintf("id=%s", id)
	_, err = es.server.MakeRequest(route, params)

	// Check for errors while sending request
	if err != nil {
		return err
	}

	logger.Info("DeleteEntityByID successful", zap.String("id", id))
	return nil
}
