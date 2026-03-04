package services

import (
	"dns-api-go/internal/bluecat"
	"dns-api-go/internal/common"
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"net/url"
	"strings"
)

// entityTypeToV2Resource maps Bluecat entity type names to V2 API resource paths.
func entityTypeToV2Resource(entityType string) string {
	switch entityType {
	case types.CONFIGURATION:
		return "configurations"
	case types.ZONE:
		return "zones"
	case types.HOSTRECORD:
		return "resourceRecords" // host records are under resourceRecords in V2
	case types.CNAMERECORD:
		return "resourceRecords"
	case types.EXTERNALHOST:
		return "resourceRecords"
	case types.GENERICRECORD:
		return "resourceRecords"
	case types.MXRECORD:
		return "resourceRecords"
	case types.TXTRECORD:
		return "resourceRecords"
	case types.SRVRECORD:
		return "resourceRecords"
	case types.HINFORECORD:
		return "resourceRecords"
	case types.IP4ADDRESS:
		return "ipv4Addresses"
	case types.IP4BLOCK:
		return "ipv4Blocks"
	case types.IP4NETWORK:
		return "ipv4Networks"
	case types.MACADDRESS:
		return "macAddresses"
	case types.MACPOOL:
		return "macPools"
	case types.VIEW:
		return "views"
	case types.DHCP4RANGE:
		return "dhcpRanges"
	default:
		return "entities"
	}
}

// GetConfigID retrieves the configuration ID from Bluecat via V2 API.
func GetConfigID(server interfaces.ServerInterface) (int, error) {
	logger.Info("GetConfigID started")

	route := "/api/v2/configurations"
	params := "limit=1&offset=0"
	resp, err := server.MakeRequest("GET", route, params, nil)
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, fmt.Errorf("failed to retrieve configuration: not found")
	}

	var collection bluecat.V2Collection
	if err := json.Unmarshal(resp, &collection); err != nil {
		logger.Error("Error unmarshalling configurations response", zap.Error(err))
		return 0, err
	}
	if len(collection.Data) == 0 {
		return 0, fmt.Errorf("failed to retrieve containerId")
	}

	configId := collection.Data[0].ID
	logger.Info("GetConfigID successful", zap.Int("configId", configId))
	return configId, nil
}

// GetParentID retrieves the parent ID of an entity from Bluecat via V2 API.
func GetParentID(server interfaces.ServerInterface, entityId int) (int, error) {
	logger.Info("GetParentID started", zap.Int("entityId", entityId))

	// In V2, get the entity and extract parent link
	route := fmt.Sprintf("/api/v2/entities/%d", entityId)
	resp, err := server.MakeRequest("GET", route, "", nil)
	if err != nil {
		logger.Error("Error getting parent ID", zap.Error(err), zap.Int("entityId", entityId))
		return -1, err
	}
	if resp == nil {
		logger.Info("Entity not found", zap.Int("entity id", entityId))
		return -1, &ErrEntityNotFound{}
	}

	var v2Entity bluecat.V2Entity
	if err := json.Unmarshal(resp, &v2Entity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return -1, err
	}

	if v2Entity.IsEmpty() {
		return -1, &ErrEntityNotFound{}
	}

	// Extract parent from the V2 response
	if v2Entity.Parent != nil {
		logger.Info("GetParentID successful", zap.Int("parentId", v2Entity.Parent.ID))
		return v2Entity.Parent.ID, nil
	}

	// If no parent link, try the _links or fall back
	return -1, &ErrEntityNotFound{}
}

// GetEntityByID retrieves an entity by ID from Bluecat via V2 API.
func GetEntityByID(server interfaces.ServerInterface, id int, includeHA bool, expectedTypes []string) (*models.Entity, error) {
	route := fmt.Sprintf("/api/v2/entities/%d", id)
	resp, err := server.MakeRequest("GET", route, "", nil)
	if err != nil {
		logger.Error("Error getting entity by ID", zap.Error(err), zap.Int("id", id))
		return nil, err
	}
	if resp == nil {
		logger.Info("Entity not found", zap.Int("id", id))
		return nil, &ErrEntityNotFound{}
	}
	logger.Info("Received response for GetEntityByID", zap.ByteString("response", resp))

	var v2Entity bluecat.V2Entity
	if err := json.Unmarshal(resp, &v2Entity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}
	if v2Entity.IsEmpty() {
		logger.Info("Entity not found", zap.Int("id", id))
		return nil, &ErrEntityNotFound{}
	}

	entity := v2Entity.ToEntity()

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

var ALLOWDELETE = []string{
	types.HOSTRECORD,
	types.EXTERNALHOST,
	types.CNAMERECORD,
	types.IP4ADDRESS,
	types.MACADDRESS,
	types.MACPOOL,
}

// DeleteEntityByID deletes an entity by ID from Bluecat via V2 API.
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

	// V2: DELETE /api/v2/{resourceType}/{id}
	resource := entityTypeToV2Resource(entity.Type)
	route := fmt.Sprintf("/api/v2/%s/%d", resource, id)
	_, err = server.MakeRequest("DELETE", route, "", nil)
	if err != nil {
		logger.Error("Error deleting entity", zap.Error(err), zap.Int("id", id))
		return err
	}

	logger.Info("DeleteEntityByID successful", zap.Int("id", id))
	return nil
}

// UpdateEntity updates an entity in Bluecat via V2 API.
func UpdateEntity(server interfaces.ServerInterface, entity *models.Entity) error {
	logger.Info("UpdateEntity started", zap.Int("entityID", entity.ID))

	// Build V2 JSON body
	v2Body := map[string]interface{}{
		"id":         entity.ID,
		"name":       entity.Name,
		"type":       entity.Type,
		"properties": entity.Properties,
	}
	bodyJSON, err := json.Marshal(v2Body)
	if err != nil {
		logger.Error("Error marshalling entity to JSON", zap.Error(err))
		return err
	}

	// V2: PUT /api/v2/{resourceType}/{id}
	resource := entityTypeToV2Resource(entity.Type)
	route := fmt.Sprintf("/api/v2/%s/%d", resource, entity.ID)
	body := strings.NewReader(string(bodyJSON))
	_, err = server.MakeRequest("PUT", route, "", body)
	if err != nil {
		logger.Error("Error updating entity", zap.Error(err), zap.Int("entityID", entity.ID))
		return err
	}

	logger.Info("UpdateEntity successful", zap.Int("entityID", entity.ID))
	return nil
}

// GetEntitiesByHintHelper retrieves entities by hint using V2 search.
func GetEntitiesByHintHelper(server interfaces.ServerInterface, resourceType string, start int, count int, options map[string]string) (*[]models.Entity, error) {
	logger.Info("GetEntitiesByHint started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Any("options", options))

	// V2: GET /api/v2/{resourceType}?filter=name:contains('{hint}')
	v2Route := fmt.Sprintf("/api/v2/%s", resourceType)

	// Build V2 query parameters
	params := fmt.Sprintf("limit=%d&offset=%d", count, start)

	// Extract hint from options and use as filter
	if hint, ok := options["hint"]; ok && hint != "" {
		params += "&filter=" + url.QueryEscape(fmt.Sprintf("name:contains('%s')", hint))
	}

	resp, err := server.MakeRequest("GET", v2Route, params, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		entities := make([]models.Entity, 0)
		return &entities, nil
	}

	var collection bluecat.V2Collection
	if err := json.Unmarshal(resp, &collection); err != nil {
		logger.Error("Error unmarshalling entities response", zap.Error(err))
		return nil, err
	}

	entities := bluecat.ConvertV2ToEntities(collection.Data)
	logger.Info("GetEntitiesByHint successful", zap.Int("count", len(entities)))
	return &entities, nil
}


// GetEntities retrieves a list of entities from Bluecat via V2 API.
func GetEntities(server interfaces.ServerInterface, start int, count int, parentId int, entityType string, includeHA bool) (*[]models.Entity, error) {
	logger.Info("GetEntities started",
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Int("parentId", parentId),
		zap.String("entityType", entityType),
		zap.Bool("includeHA", includeHA))

	// V2: GET /api/v2/{resourceType}?limit=N&offset=N&filter=...
	resource := entityTypeToV2Resource(entityType)
	route := fmt.Sprintf("/api/v2/%s", resource)
	params := fmt.Sprintf("limit=%d&offset=%d", count, start)

	// If parentId is specified, add it as a filter
	if parentId > 0 {
		params += fmt.Sprintf("&parentId=%d", parentId)
	}

	resp, err := server.MakeRequest("GET", route, params, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		entities := make([]models.Entity, 0)
		return &entities, nil
	}

	var collection bluecat.V2Collection
	if err := json.Unmarshal(resp, &collection); err != nil {
		logger.Error("Error unmarshalling entities response", zap.Error(err))
		return nil, err
	}

	entities := bluecat.ConvertV2ToEntities(collection.Data)
	logger.Info("GetEntities successful", zap.Int("count", len(entities)))
	return &entities, nil
}

// GetEntityByName retrieves an entity by name from Bluecat via V2 API.
func GetEntityByName(server interfaces.ServerInterface, name string, entityType string, parentId int, includeHA bool) (*models.Entity, error) {
	logger.Info("GetEntityByName started", zap.String("name", name), zap.String("entityType", entityType))

	// V2: GET /api/v2/{resourceType}?filter=name:eq('{name}')
	resource := entityTypeToV2Resource(entityType)
	route := fmt.Sprintf("/api/v2/%s", resource)
	params := fmt.Sprintf("filter=%s&limit=1", url.QueryEscape(fmt.Sprintf("name:eq('%s')", name)))
	if parentId > 0 {
		params += fmt.Sprintf("&parentId=%d", parentId)
	}

	resp, err := server.MakeRequest("GET", route, params, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		logger.Info("Entity not found", zap.String("name", name))
		return nil, &ErrEntityNotFound{}
	}

	var collection bluecat.V2Collection
	if err := json.Unmarshal(resp, &collection); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}

	if len(collection.Data) == 0 {
		logger.Info("Entity not found", zap.String("name", name))
		return nil, &ErrEntityNotFound{}
	}

	entity := collection.Data[0].ToEntity()
	logger.Info("GetEntityByName successful", zap.Int("entityID", entity.ID))
	return &entity, nil
}

// searchObjectByTypes searches for entities by keyword and types via V2 API.
func searchObjectByTypes(server interfaces.ServerInterface, keyword string, start int, count int, includeHA bool, entityTypes []string) (*[]models.Entity, error) {
	logger.Info("searchObjectByTypes started",
		zap.String("keyword", keyword),
		zap.Int("start", start),
		zap.Int("count", count),
		zap.Strings("types", entityTypes))

	// V2: GET /api/v2/search?keyword=...&types=...&limit=N&offset=N
	route := "/api/v2/search"
	typesStr := strings.Join(entityTypes, ",")
	params := fmt.Sprintf("keyword=%s&types=%s&limit=%d&offset=%d",
		url.QueryEscape(keyword), url.QueryEscape(typesStr), count, start)

	resp, err := server.MakeRequest("GET", route, params, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		entities := make([]models.Entity, 0)
		return &entities, nil
	}

	var collection bluecat.V2Collection
	if err := json.Unmarshal(resp, &collection); err != nil {
		logger.Error("Error unmarshalling entities response", zap.Error(err))
		return nil, err
	}

	entities := bluecat.ConvertV2ToEntities(collection.Data)
	logger.Info("searchObjectByTypes successful", zap.Int("count", len(entities)))
	return &entities, nil
}