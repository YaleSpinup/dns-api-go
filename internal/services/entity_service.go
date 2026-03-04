package services

import (
	"dns-api-go/internal/bluecat"
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
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

	// V2: GET /api/v2/{resourceType}?limit=N&offset=N&filter=...
	resource := entityTypeToV2Resource(objectType)
	route := fmt.Sprintf("/api/v2/%s", resource)
	queryParams := url.Values{}
	queryParams.Set("limit", fmt.Sprintf("%d", count))
	queryParams.Set("offset", fmt.Sprintf("%d", start))

	// Build V2 filter from the filters map
	var filterParts []string
	for key, value := range filters {
		filterParts = append(filterParts, fmt.Sprintf("%s:eq('%s')", key, value))
	}
	if len(filterParts) > 0 {
		queryParams.Set("filter", fmt.Sprintf("%s", joinFilters(filterParts)))
	}

	// Send http request to bluecat
	resp, err := es.server.MakeRequest("GET", route, queryParams.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		entities := make([]models.Entity, 0)
		return &entities, nil
	}

	// Unmarshal V2 collection response
	var collection bluecat.V2Collection
	if err := json.Unmarshal(resp, &collection); err != nil {
		logger.Error("Error unmarshalling entities response", zap.Error(err))
		return nil, err
	}

	entities := bluecat.ConvertV2ToEntities(collection.Data)

	logger.Info("CustomSearch successful", zap.Int("count", len(entities)))
	return &entities, nil
}

// joinFilters joins filter expressions with " and " for V2 API.
func joinFilters(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " and "
		}
		result += p
	}
	return result
}
