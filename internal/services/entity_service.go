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

type EntityGetter interface {
	GetEntity(id int, includeHA bool) (*models.Entity, error)
}

type EntityDeleter interface {
	DeleteEntity(id int, expectedTypes []string) error
}

type EntitiesLister interface {
	GetEntitiesByHint(start int, count int, options map[string]string) (*[]models.Entity, error)
}

type BaseService struct {
	server interfaces.ServerInterface
}

// NewBaseService Constructor for BaseService
func NewBaseService(server interfaces.ServerInterface) *BaseService {
	return &BaseService{server: server}
}

// GetEntity Retrieves an entity by ID from bluecat
func (es *BaseService) GetEntity(id int, includeHA bool) (*models.Entity, error) {
	logger.Info("GetEntity started", zap.Int("id", id), zap.Bool("includeHA", includeHA))

	// Send http request to bluecat
	route, params := "/getEntityById", fmt.Sprintf("id=%d&includeHA=%t", id, includeHA)
	resp, err := es.server.MakeRequest("GET", route, params)

	// Check for errors when sending request
	if err != nil {
		return nil, err
	}
	logger.Info("Received response for GetEntityByID", zap.ByteString("response", resp))

	// Unmarshal the response
	var entityResp models.EntityResponse
	if err := json.Unmarshal(resp, &entityResp); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}
	// Check if the response represents an empty entity
	if entityResp.IsEmpty() {
		logger.Info("Entity not found", zap.Int("id", id))
		return nil, &ErrEntityNotFound{}
	}

	// Convert EntityResponse to Entity
	entity := entityResp.ToEntity()

	logger.Info("GetEntity successful",
		zap.Int("entityID", entity.ID),
		zap.String("entityType", entity.Type))
	return &entity, nil
}

var ALLOWDELETE = []string{
	types.HOSTRECORD,
	types.EXTERNALHOST,
	types.CNAMERECORD,
	types.IP4ADDRESS,
	types.MACADDRESS,
	types.MACPOOL,
}

// DeleteEntity Deletes an entity by ID from bluecat
func (es *BaseService) DeleteEntity(id int, expectedTypes []string) error {
	logger.Info("DeleteEntity started", zap.Int("id", id))

	// Get the entity type
	entity, err := es.GetEntity(id, false)
	if err != nil {
		return err
	}

	// If the entity type in Bluecat is not among the expected types, do not delete it
	if len(expectedTypes) > 0 && !common.Contains(expectedTypes, entity.Type) {
		logger.Info("Entity type does not match expected types",
			zap.Int("id", id),
			zap.String("type", entity.Type),
			zap.Any("expectedTypes", expectedTypes))

		return &ErrEntityTypeMismatch{expectedTypes, entity.Type}
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
	route, params := "/delete", fmt.Sprintf("objectId=%d", id)
	_, err = es.server.MakeRequest("DELETE", route, params)

	// Check for errors while sending request
	if err != nil {
		return err
	}

	logger.Info("DeleteEntity successful", zap.Int("id", id))
	return nil
}

// GetEntities retrieves a list of entities from Bluecat based on the provided parameters.
// Note: The maximum value for count is 10.
func (es *BaseService) GetEntities(start int, count int, parentId int, entityType string, includeHA bool) (*[]models.Entity, error) {
	logger.Info("GetEntities started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Int("parentId", parentId),
		zap.String("entityType", entityType),
		zap.Bool("includeHA", includeHA))

	// Send http request to bluecat
	route := "/getEntities"
	params := fmt.Sprintf("start=%d&count=%d&parentId=%d&type=%s&includeHA=%t",
		start, count, parentId, entityType, includeHA)
	resp, err := es.server.MakeRequest("GET", route, params)

	// Check for errors when sending request
	if err != nil {
		return nil, err
	}

	// Unmarshal the response
	var entitiesResp []models.EntityResponse
	if err := json.Unmarshal(resp, &entitiesResp); err != nil {
		logger.Error("Error unmarshalling entities response", zap.Error(err))
		return nil, err
	}

	// For each entity response, convert it to an entity
	entities := models.ConvertToEntities(entitiesResp)

	logger.Info("GetEntities successful", zap.Int("count", len(entities)))
	return &entities, nil
}
