package services

import (
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"strings"
)

type ZoneEntityService interface {
	GetZones(start int, count int, options map[string]string) (*[]models.Entity, error)
}

type ZoneService struct {
	server interfaces.ServerInterface
	EntitiesLister
}

// NewZoneService Constructor for ZoneService
func NewZoneService(server interfaces.ServerInterface, entitiesLister EntitiesLister) *ZoneService {
	return &ZoneService{server: server, EntitiesLister: entitiesLister}
}

// GetZones Retrieves zones from bluecat
func (zs *ZoneService) GetZones(start int, count int, options map[string]string) (*[]models.Entity, error) {
	logger.Info("GetZones started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Any("options", options))

	// Retrieve Configuration Id
	containers, err := zs.GetEntities(0, 1, 0, "Configuration", false)
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
	var zonesResp []EntityResponse
	if err := json.Unmarshal(resp, &zonesResp); err != nil {
		logger.Error("Error unmarshalling zones response", zap.Error(err))
		return nil, err
	}

	// For each zone entity response, convert it to an entity
	zones := make([]models.Entity, 0, len(zonesResp))
	for i, zoneResp := range zonesResp {
		zones[i] = *zoneResp.ToEntity()
	}

	logger.Info("GetZones successful", zap.Int("count", len(zones)))
	return &zones, nil
}
