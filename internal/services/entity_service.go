package services

import (
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"go.uber.org/zap"
	"net/url"
)

type BaseEntityService interface {
	GetEntity(id int, includeHA bool) (*models.Entity, error)
	DeleteEntity(id int) error
}

type BaseService struct {
	server interfaces.ServerInterface
}

// NewBaseService Constructor for BaseService
func NewBaseService(server interfaces.ServerInterface) *BaseService {
	return &BaseService{server: server}
}

func (es *BaseService) GetEntity(id int, includeHA bool) (*models.Entity, error) {
	logger.Info("GetEntity started", zap.Int("id", id), zap.Bool("includeHA", includeHA))

	entity, err := GetEntityByID(es.server, id, includeHA, nil)
	if err != nil {
		return nil, err
	}

	logger.Info("GetEntity successful",
		zap.Int("entityID", entity.ID),
		zap.String("entityType", entity.Type))

	return entity, nil
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
func (es *BaseService) DeleteEntity(id int) error {
	logger.Info("DeleteEntity started", zap.Int("id", id))

	err := DeleteEntityByID(es.server, id, nil)
	if err != nil {
		return err
	}

	logger.Info("DeleteEntity successful", zap.Int("id", id))
	return nil
}

func (es *BaseService) CustomSearch(start int, count int, filters map[string]string, options []string, objectType string) (*[]models.Entity, error) {
	logger.Info("CustomSearch started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Any("filters", filters),
		zap.Any("options", options),
		zap.String("objectType", objectType))

	// Construct route and query parameters
	route := "/customSearch"
	queryParams := url.Values{}

	logger.Info("CustomSearch successful", zap.Int("count", len(*entities)))
	return entities, nil
}
