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

// GetConfigID retrieves the configuration ID from Bluecat.
func GetConfigID(server interfaces.ServerInterface) (int, error) {
	logger.Info("GetConfigID started")

	baseService := NewBaseService(server)
	containers, err := baseService.GetEntities(0, 1, 0, types.CONFIGURATION, false)
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
	resp, err := server.MakeRequest("GET", route, params)
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

	logger.Info("GetEntitiesByHint successful", zap.Int("count", len(entities)))
	return &entities, nil
}
