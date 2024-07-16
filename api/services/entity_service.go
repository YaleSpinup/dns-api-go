package services

import (
	"dns-api-go/api"
	"encoding/json"
	"fmt"
)

type Entity struct {
	ID         int
	Name       string
	Type       string
	Properties string
}

type EntityResponse struct {
	ID         int     `json:"id"`
	Name       *string `json:"name"`
	Type       *string `json:"type"`
	Properties *string `json:"properties"`
}

type EntityService interface {
	GetEntityByID(id string, includeHA string) (*Entity, error)
	DeleteEntityByID(id string) error
}

type GenericEntityService struct {
	server api.ServerInterface
}

// NewGenericEntityService Constructor for GenericEntityService
func NewGenericEntityService(server api.ServerInterface) *GenericEntityService {
	return &GenericEntityService{server: server}
}

func (es *GenericEntityService) GetEntityByID(id string, includeHA string) (*Entity, error) {
	// Send http request to bluecat
	route, params := "/getEntityById", fmt.Sprintf("id=%s&includeHA=%s", id, includeHA)
	resp, err := es.server.MakeRequest(route, params)

	// Check for errors when sending request
	if err != nil {
		return nil, err
	}

	// Unmarshal the response
	var entityResp EntityResponse
	if err := json.Unmarshal(resp, &entityResp); err != nil {
		return nil, err
	}
	// Check if the response represents an empty entity
	if entityResp.ID == 0 && entityResp.Name == nil && entityResp.Type == nil && entityResp.Properties == nil {
		return nil, fmt.Errorf("entity not found")
	}

	// Convert EntityResponse to Entity
	entity := &Entity{
		ID:         entityResp.ID,
		Name:       *entityResp.Name,
		Type:       *entityResp.Type,
		Properties: *entityResp.Properties,
	}

	return entity, nil
}

func (es *GenericEntityService) DeleteEntityByID(id string) error {
	return nil
}
