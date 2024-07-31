package services

import (
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
)

type EntityResponse struct {
	ID         int     `json:"id"`
	Name       *string `json:"name"`
	Type       *string `json:"type"`
	Properties *string `json:"properties"`
}

type GenericEntityService struct {
	server interfaces.ServerInterface
}

// NewGenericEntityService Constructor for GenericEntityService
func NewGenericEntityService(server interfaces.ServerInterface) *GenericEntityService {
	return &GenericEntityService{server: server}
}

// GetEntityByID Retrieves an entity by ID from bluecat
func (es *GenericEntityService) GetEntityByID(id int, includeHA bool) (*models.Entity, error) {
	logger.Info("GetEntityByID started", zap.Int("id", id), zap.Bool("includeHA", includeHA))

	// Send http request to bluecat
	route, params := "/getEntityById", fmt.Sprintf("id=%d&includeHA=%t", id, includeHA)
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
		logger.Info("Entity not found", zap.Int("id", id))
		return nil, &ErrEntityNotFound{}
	}

	// Convert EntityResponse to Entity
	entity := entityResp.ToEntity()

	logger.Info("GetEntityByID successful",
		zap.Int("entityID", entity.ID),
		zap.String("entityType", entity.Type))
	return entity, nil
}

// ToEntity Converts EntityResponse to Entity
func (er *EntityResponse) ToEntity() *models.Entity {
	// Handle nil pointer dereference if name or properties is null
	// Type is guaranteed to be non-nil unless entity does not exist, in which case it is handled earlier
	var name, properties string
	if er.Name != nil {
		name = *er.Name
	}
	if er.Properties != nil {
		properties = *er.Properties
	}

	// Convert EntityResponse to Entity
	entity := &models.Entity{
		ID:         er.ID,
		Name:       name,
		Type:       *er.Type,
		Properties: properties,
	}

	return entity
}

var ALLOWDELETE = []string{
	"HostRecord",
	"ExternalHostRecord",
	"AliasRecord",
	"IP4Address",
	"MACAddress",
	"MACPool",
}

// DeleteEntityByID Deletes an entity by ID from bluecat
func (es *GenericEntityService) DeleteEntityByID(id int) error {
	logger.Info("DeleteEntityByID started", zap.Int("id", id))

	// Get the entity type
	entity, err := es.GetEntityByID(id, false)
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
		logger.Info("Entity deletion not allowed", zap.Int("id", id), zap.String("type", entity.Type))
		return &ErrDeleteNotAllowed{Type: entity.Type}
	}

	// Send http request to bluecat
	route, params := "/delete", fmt.Sprintf("id=%d", id)
	_, err = es.server.MakeRequest(route, params)

	// Check for errors while sending request
	if err != nil {
		return err
	}

	logger.Info("DeleteEntityByID successful", zap.Int("id", id))
	return nil
}
