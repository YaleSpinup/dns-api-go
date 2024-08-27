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

type NetworkEntityService interface {
	GetNetworks(start int, count int, options map[string]string) (*[]models.Entity, error)
	GetEntity(networkId int, includeHA bool) (*models.Entity, error)
}

type NetworkService struct {
	server interfaces.ServerInterface
	EntitiesLister
	EntityGetter
}

// NewNetworkService Constructor for NetworkService
func NewNetworkService(server interfaces.ServerInterface, entitiesLister EntitiesLister, entityGetter EntityGetter) *NetworkService {
	return &NetworkService{server: server, EntitiesLister: entitiesLister, EntityGetter: entityGetter}
}

// GetNetworks Retrieves a list of networks from bluecat
// Note: The maximum that count can be is 10.
func (ns *NetworkService) GetNetworks(start int, count int, options map[string]string) (*[]models.Entity, error) {
	logger.Info("GetNetworks started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Any("options", options))

	// Retrieve Configuration Id
	containers, err := ns.GetEntities(0, 1, 0, types.CONFIGURATION, false)
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

	// Use the configuration ID to call the Bluecat API to get networks
	route := "/getIP4NetworksByHint"
	resp, err := ns.server.MakeRequest("GET", route, params)
	if err != nil {
		return nil, err
	}

	// Unmarshal the response
	var networkResp []models.EntityResponse
	if err := json.Unmarshal(resp, &networkResp); err != nil {
		logger.Error("Error unmarshalling networks response", zap.Error(err))
		return nil, err
	}

	// For each network entity response, convert it to an entity
	networks := models.ConvertToEntities(networkResp)

	logger.Info("GetNetworks successful", zap.Int("count", len(networks)))
	return &networks, nil
}
