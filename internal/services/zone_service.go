package services

import (
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"strings"
)

type ZoneEntityService interface {
	GetZones(start int, count int, options map[string]string) (*[]models.Entity, error)
	GetEntity(zoneId int, includeHA bool) (*models.Entity, error)
}

type ZoneService struct {
	server interfaces.ServerInterface
	EntitiesLister
	EntityGetter
}

// NewZoneService Constructor for ZoneService
func NewZoneService(server interfaces.ServerInterface, entitiesLister EntitiesLister, entityGetter EntityGetter) *ZoneService {
	return &ZoneService{server: server, EntitiesLister: entitiesLister, EntityGetter: entityGetter}
}

// GetZones Retrieves zones from bluecat
func (zs *ZoneService) GetZones(start int, count int, options map[string]string) (*[]models.Entity, error) {
	logger.Info("GetZones started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Any("options", options))

	// Retrieve Configuration Id
	containers, err := zs.GetEntities(0, 1, 0, types.CONFIGURATION, false)
	if err != nil {
		return nil, err
	}
	if len(*containers) == 0 {
		return nil, fmt.Errorf("failed to retrieve containerId")
	}
	configId := (*containers)[0].ID

	// Construct the request parameters
	params := fmt.Sprintf("containerId=%d&start=%d&count=%d", configId, start, count)
	if len(options) > 0 {
		opts := make([]string, 0, len(options))
		for key, value := range options {
			opts = append(opts, fmt.Sprintf("%s=%s", key, value))
		}
		params += "&options=" + strings.Join(opts, "|")
	}

	// Use the configuration ID to call the Bluecat API to get zones
	route := "/getZonesByHint"
	resp, err := zs.server.MakeRequest("GET", route, params)
	if err != nil {
		return nil, err
	}

	// Unmarshal the response
	var zonesResp []models.EntityResponse
	if err := json.Unmarshal(resp, &zonesResp); err != nil {
		logger.Error("Error unmarshalling zones response", zap.Error(err))
		return nil, err
	}

	// For each zone entity response, convert it to an entity
	zones := models.ConvertToEntities(zonesResp)

	logger.Info("GetZones successful", zap.Int("count", len(zones)))
	return &zones, nil
}

func (zs *ZoneService) GetEntity(zoneId int, includeHA bool) (*models.Entity, error) {
	logger.Info("GetZone started", zap.Int("zoneId", zoneId))

	// Call EntityGetter
	entity, err := zs.EntityGetter.GetEntity(zoneId, includeHA)
	if err != nil {
		return nil, err
	}

	// Check if the entity type is a zone
	if entity.Type != types.ZONE {
		return nil, &ErrEntityTypeMismatch{ExpectedTypes: []string{types.ZONE}, ActualType: entity.Type}
	}

	logger.Info("GetZone successful",
		zap.Int("entityId", entity.ID),
		zap.String("entityType", entity.Type))
	return entity, nil
}
