package services

import (
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"go.uber.org/zap"
)

type ZoneEntityService interface {
	GetZones(start int, count int, options map[string]string) (*[]models.Entity, error)
	GetEntity(zoneId int, includeHA bool) (*models.Entity, error)
}

type ZoneService struct {
	server interfaces.ServerInterface
	interfaces.EntityGetter
}

// NewZoneService Constructor for ZoneService
func NewZoneService(server interfaces.ServerInterface, entityGetter interfaces.EntityGetter) *ZoneService {
	return &ZoneService{server: server, EntityGetter: entityGetter}
}

// GetEntitiesByHint Retrieves zones from bluecat
// Note: The maximum that count can be is 10.
func (zs *ZoneService) GetEntitiesByHint(start int, count int, options map[string]string) (*[]models.Entity, error) {
	logger.Info("GetEntitiesByHint started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Any("options", options))

	route := "/getZonesByHint"
	zones, err := GetEntitiesByHintHelper(zs.server, route, start, count, options)
	if err != nil {
		return nil, err
	}

	logger.Info("GetEntitiesByHint successful", zap.Int("count", len(*zones)))
	return zones, nil
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
