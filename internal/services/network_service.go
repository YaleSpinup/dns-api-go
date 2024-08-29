package services

import (
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"go.uber.org/zap"
)

type NetworkEntityService interface {
	GetEntityByHint(start int, count int, options map[string]string) (*[]models.Entity, error)
	GetEntity(networkId int, includeHA bool) (*models.Entity, error)
}

type NetworkService struct {
	server interfaces.ServerInterface
	interfaces.EntityGetter
}

// NewNetworkService Constructor for NetworkService
func NewNetworkService(server interfaces.ServerInterface, entityGetter interfaces.EntityGetter) *NetworkService {
	return &NetworkService{server: server, EntityGetter: entityGetter}
}

// GetEntitiesByHint Retrieves a list of networks from bluecat
// Note: The maximum that count can be is 10.
func (ns *NetworkService) GetEntitiesByHint(start int, count int, options map[string]string) (*[]models.Entity, error) {
	logger.Info("GetEntitiesByHint started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Any("options", options))

	route := "/getIP4NetworksByHint"
	networks, err := GetEntitiesByHintHelper(ns.server, route, start, count, options)
	if err != nil {
		return nil, err
	}

	logger.Info("GetEntitiesByHint successful", zap.Int("count", len(*networks)))
	return networks, nil
}

func (ns *NetworkService) GetEntity(networkId int, includeHA bool) (*models.Entity, error) {
	logger.Info("GetNetwork started", zap.Int("networkId", networkId))

	// Call EntityGetter
	entity, err := ns.EntityGetter.GetEntity(networkId, includeHA)
	if err != nil {
		return nil, err
	}

	// Check if the entity type is a network
	if entity.Type != types.NETWORK {
		return nil, &ErrEntityTypeMismatch{ExpectedTypes: []string{types.NETWORK}, ActualType: entity.Type}
	}

	logger.Info("GetNetwork successful",
		zap.Int("entityId", entity.ID),
		zap.String("entityType", entity.Type))
	return entity, nil
}
