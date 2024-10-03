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
	"strings"
)

// GetConfigID retrieves the configuration ID from Bluecat.
func GetConfigID(server interfaces.ServerInterface) (int, error) {
	logger.Info("GetConfigID started")

	containers, err := GetEntities(server, 0, 1, 0, types.CONFIGURATION, false)
	if err != nil {
		return 0, err
	}
	if len(*containers) == 0 {
		return 0, fmt.Errorf("failed to retrieve containerId")
	}
	configId := (*containers)[0].ID

	logger.Info("GetConfigID successful", zap.Int("configId", configId))
	return configId, nil
}

// GetParentID retrieves the parent ID of an entity from Bluecat.
func GetParentID(server interfaces.ServerInterface, entityId int) (int, error) {
	logger.Info("GetParentID started", zap.Int("entityId", entityId))

	// Send http request to bluecat
	route, params := "/getParent", fmt.Sprintf("entityId=%d", entityId)
	resp, err := server.MakeRequest("GET", route, params, nil)
	if err != nil {
		logger.Error("Error getting parent ID", zap.Error(err), zap.Int("entityId", entityId))
		return -1, err
	}

	// Unmarshal the response
	var bluecatEntity models.BluecatEntity
	if err := json.Unmarshal(resp, &bluecatEntity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return -1, err
	}

	// Check if the response represents an empty entity
	if bluecatEntity.IsEmpty() {
		logger.Info("Entity not found", zap.Int("entity id", entityId))
		return -1, &ErrEntityNotFound{}
	}

	// Convert BluecatEntity to Entity
	parentEntity := bluecatEntity.ToEntity()

	logger.Info("GetParentID successful", zap.Int("parentId", parentEntity.ID))
	return parentEntity.ID, nil
}

// GetEntityByID Retrieves an entity by ID from bluecat
func GetEntityByID(server interfaces.ServerInterface, id int, includeHA bool, expectedTypes []string) (*models.Entity, error) {
	// Send http request to bluecat
	route, params := "/getEntityById", fmt.Sprintf("id=%d&includeHA=%t", id, includeHA)
	resp, err := server.MakeRequest("GET", route, params, nil)

	// Check for errors when sending request
	if err != nil {
		logger.Error("Error getting entity by ID", zap.Error(err), zap.Int("id", id))
		return nil, err
	}
	logger.Info("Received response for GetEntityByID", zap.ByteString("response", resp))

	// Unmarshal the response
	var bluecatEntity models.BluecatEntity
	if err := json.Unmarshal(resp, &bluecatEntity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}
	// Check if the response represents an empty entity
	if bluecatEntity.IsEmpty() {
		logger.Info("Entity not found", zap.Int("id", id))
		return nil, &ErrEntityNotFound{}
	}

	// Convert BluecatEntity to Entity
	entity := bluecatEntity.ToEntity()

	// Check if the entity type is one of the expected types
	if len(expectedTypes) > 0 && !common.Contains(expectedTypes, entity.Type) {
		logger.Error("Entity type does not match expected types",
			zap.String("entityType", entity.Type),
			zap.Strings("expectedTypes", expectedTypes))
		return nil, &ErrEntityTypeMismatch{expectedTypes, entity.Type}
	}

	logger.Info("GetEntityByID successful",
		zap.Int("entityID", entity.ID),
		zap.String("entityType", entity.Type))
	return &entity, nil
}

// DeleteEntityByID Deletes an entity by ID from bluecat
func DeleteEntityByID(server interfaces.ServerInterface, id int, expectedTypes []string) error {
	logger.Info("DeleteEntityByID started", zap.Int("id", id))

	// Get the entity type
	entity, err := GetEntityByID(server, id, false, expectedTypes)
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
	route, params := "/delete", fmt.Sprintf("objectId=%d", id)
	_, err = server.MakeRequest("DELETE", route, params, nil)

	// Check for errors while sending request
	if err != nil {
		logger.Error("Error deleting entity", zap.Error(err), zap.Int("id", id))
		return err
	}

	logger.Info("DeleteEntityByID successful", zap.Int("id", id))
	return nil
}

// UpdateEntity Updates an entity in Bluecat
func UpdateEntity(server interfaces.ServerInterface, entity *models.Entity) error {
	logger.Info("UpdateEntity started", zap.Int("entityID", entity.ID))

	bluecatEntityJSON, err := entity.ToBluecatJSON()
	if err != nil {
		logger.Error("Error marshalling entity to JSON for Bluecat", zap.Error(err))
	}

	// Create an io.Reader from the JSON string
	body := strings.NewReader(string(bluecatEntityJSON))

	// Send http request to bluecat
	route := "/update"
	_, err = server.MakeRequest("PUT", route, "", body)

	// Check for errors when sending request
	if err != nil {
		logger.Error("Error updating entity", zap.Error(err), zap.Int("entityID", entity.ID))
		return err
	}

	logger.Info("UpdateEntity successful", zap.Int("entityID", entity.ID))
	return nil
}

// GetEntitiesByHintHelper retrieves entities by hint, given a specific route.
// Many of the entity retrieval functions in across the different services use this helper function because they share the same logic
func GetEntitiesByHintHelper(server interfaces.ServerInterface, route string, start int, count int, options map[string]string) (*[]models.Entity, error) {
	logger.Info("GetEntitiesByHint started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Any("options", options))

	// Use Configuration ID as the container ID
	containerId, err := GetConfigID(server)
	if err != nil {
		return nil, err
	}

	// Construct the request parameters
	params := fmt.Sprintf("containerId=%d&start=%d&count=%d", containerId, start, count)
	params += "&options=" + common.ConvertToSeparatedString(options, "|")

	// Use the configuration ID to call the Bluecat API to get entities
	resp, err := server.MakeRequest("GET", route, params, nil)
	if err != nil {
		return nil, err
	}

	// Unmarshal the response
	var entitiesResp []models.BluecatEntity
	if err := json.Unmarshal(resp, &entitiesResp); err != nil {
		logger.Error("Error unmarshalling entities response", zap.Error(err))
		return nil, err
	}

	// For each entity response, convert it to an entity
	entities := models.ConvertToEntities(entitiesResp)

	logger.Info("GetEntitiesByHint successful", zap.Int("count", len(entities)))
	return &entities, nil
}

// GetEntities retrieves a list of entities from Bluecat based on the provided parameters.
// Note: The maximum value for count is 10.
func GetEntities(server interfaces.ServerInterface, start int, count int, parentId int, entityType string, includeHA bool) (*[]models.Entity, error) {
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
	resp, err := server.MakeRequest("GET", route, params, nil)

	// Check for errors when sending request
	if err != nil {
		return nil, err
	}

	// Unmarshal the response
	var entitiesResp []models.BluecatEntity
	if err := json.Unmarshal(resp, &entitiesResp); err != nil {
		logger.Error("Error unmarshalling entities response", zap.Error(err))
		return nil, err
	}

	// For each entity response, convert it to an entity
	entities := models.ConvertToEntities(entitiesResp)

	logger.Info("GetEntities successful", zap.Int("count", len(entities)))
	return &entities, nil
}
