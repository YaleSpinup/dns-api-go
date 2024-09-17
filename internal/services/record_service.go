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

type RecordEntityService interface {
	GetEntity(recordId int, includeHA bool) (*models.Entity, error)
	GetRecordsByType(recordType string, parameters map[string]interface{}, viewId int) (*[]models.Entity, error)
	CreateRecord(recordType string, parameters map[string]interface{}, viewId int) (*models.Entity, error)
	DeleteEntity(recordId int) error
}

type RecordService struct {
	server interfaces.ServerInterface
}

// NewRecordService Constructor for RecordService
func NewRecordService(server interfaces.ServerInterface) *RecordService {
	return &RecordService{server: server}
}

func (rs *RecordService) GetEntity(recordId int, includeHA bool) (*models.Entity, error) {
	logger.Info("RecordService GetEntity started", zap.Int("recordId", recordId))

	// Call EntityGetter
	entity, err := GetEntityByID(rs.server, recordId, includeHA, []string{types.CNAMERECORD, types.HOSTRECORD, types.EXTERNALHOST})
	if err != nil {
		return nil, err
	}

	logger.Info("GetEntity successful",
		zap.Int("entityId", entity.ID),
		zap.String("entityType", entity.Type))
	return entity, nil
}

func (rs *RecordService) DeleteEntity(recordId int) error {
	logger.Info("RecordService DeleteEntity started", zap.Int("recordId", recordId))

	// Call EntityDeleter
	err := DeleteEntityByID(rs.server, recordId, []string{types.CNAMERECORD, types.HOSTRECORD, types.EXTERNALHOST})
	if err != nil {
		return err
	}

	logger.Info("DeleteEntity successful", zap.Int("recordId", recordId))
	return nil
}

func (rs *RecordService) GetRecordsByType(recordType string, parameters map[string]interface{}, viewId int) (*[]models.Entity, error) {
	logger.Info("RecordService GetRecordByType started", zap.String("recordType", recordType))

	// Validate common parameters
	count, ok := parameters["count"].(int)
	if !ok {
		return nil, fmt.Errorf("invalid type for count")
	}
	start, ok := parameters["start"].(int)
	if !ok {
		return nil, fmt.Errorf("invalid type for start")
	}

	var entities *[]models.Entity
	var err error
	switch recordType {
	case types.HOSTRECORD, types.CNAMERECORD:
		// Validate the parameters
		options, ok := parameters["options"].(map[string]string)
		if !ok {
			return nil, fmt.Errorf("invalid type for options")
		}

		entities, err = rs.getHostOrAliasRecordsByHint(recordType, start, count, options)
	case types.EXTERNALHOST:
		// Validate the parameters
		name, ok := parameters["name"].(string)
		if !ok {
			name = ""
		}
		keyword, ok := parameters["keyword"].(string)
		if !ok {
			keyword = ""
		}

		entities, err = rs.getExternalRecord(name, keyword, start, count, false, viewId)
	default:
		return nil, fmt.Errorf("invalid record type")
	}

	// Check for error and return entities
	if err != nil {
		return nil, err
	}
	return entities, nil
}

func (rs *RecordService) getHostOrAliasRecordsByHint(recordType string, start int, count int, options map[string]string) (*[]models.Entity, error) {
	// Define route and parameter map
	var route string
	switch recordType {
	case types.HOSTRECORD:
		route = "/getHostRecordsByHint"
	case types.CNAMERECORD:
		route = "/getAliasesByHint"
	default:
		return nil, fmt.Errorf("invalid record type")
	}
	paramsMap := map[string]string{
		"count":   fmt.Sprintf("%d", count),
		"start":   fmt.Sprintf("%d", start),
		"options": common.ConvertToSeparatedString(options, "&"),
	}

	// Send request to bluecat
	params := common.ConvertToSeparatedString(paramsMap, "&")
	resp, err := rs.server.MakeRequest("GET", route, params, nil)
	if err != nil {
		return nil, err
	}

	// Unmarshal the response
	var entitiesResp []models.BluecatEntity
	if err := json.Unmarshal(resp, &entitiesResp); err != nil {
		logger.Error("Error unmarshalling entities response", zap.Error(err))
		return nil, err
	}

	// For each entity response, convert it to an entity
	entities := models.ConvertToEntities(entitiesResp)

	return &entities, nil
}

func (rs *RecordService) getExternalRecord(name string, keyword string, start int, count int, includeHA bool, viewId int) (*[]models.Entity, error) {
	if name != "" {
		// Cal GetEntityByName
		entity, err := GetEntityByName(rs.server, name, types.EXTERNALHOST, viewId, includeHA)
		if err != nil {
			return nil, err
		}
		return &[]models.Entity{*entity}, nil
	} else if keyword != "" {
		// Call searchObjectsByTypes
		entities, err := searchObjectByTypes(rs.server, keyword, start, count, includeHA, []string{types.EXTERNALHOST})
		if err != nil {
			return nil, err
		}
		return entities, nil
	} else {
		// Call GetEntities
		entities, err := GetEntities(rs.server, start, count, viewId, types.EXTERNALHOST, includeHA)
		if err != nil {
			return nil, err
		}
		return entities, nil
	}
}

func (rs *RecordService) CreateRecord(recordType string, parameters map[string]interface{}, viewId int) (*models.Entity, error) {
	logger.Info("Create Record started", zap.String("recordType", recordType))

	var route string
	var paramsMap map[string]string
	var err error

	// Set the route and properties map according to the record type
	switch recordType {
	case types.HOSTRECORD:
		route, paramsMap, err = prepCreateHostParams(parameters, viewId)
		if err != nil {
			return nil, err
		}
	case types.CNAMERECORD:
		route, paramsMap, err = prepCreateCNAMEParams(parameters, viewId)
		if err != nil {
			return nil, err
		}
	case types.EXTERNALHOST:
		route, paramsMap, err = prepCreateExternalParams(parameters, viewId)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid record type")
	}

	// Send request to bluecat
	params := common.ConvertToSeparatedString(paramsMap, "&")
	resp, err := rs.server.MakeRequest("POST", route, params, nil)
	if err != nil {
		return nil, err
	}

	// Unmarshal the response to get the object iD
	var recordId int
	if err := json.Unmarshal(resp, &recordId); err != nil {
		logger.Error("Error unmarshalling recordId", zap.Error(err))
		return nil, err
	}

	// Get the new entity details
	entity, err := rs.GetEntity(recordId, true)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func prepCreateHostParams(parameters map[string]interface{}, viewId int) (string, map[string]string, error) {
	// Validate parameters
	absoluteName, ok := parameters["absoluteName"].(string)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for absoluteName")
	}
	addresses, ok := parameters["addresses"].(map[string]string)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for addresses")
	}
	properties, ok := parameters["properties"].(map[string]string)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for properties")
	}
	ttl, ok := parameters["ttl"].(int)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for ttl")
	}

	// Define route and parameter map
	route := "/addHostRecord"
	paramsMap := map[string]string{
		"absoluteName": absoluteName,
		"addresses":    common.ConvertToSeparatedString(addresses, ","),
		"properties":   common.ConvertToSeparatedString(properties, "|"),
		"ttl":          fmt.Sprintf("%d", ttl),
		"viewId":       fmt.Sprintf("%d", viewId),
	}
	return route, paramsMap, nil
}

func prepCreateCNAMEParams(parameters map[string]interface{}, viewId int) (string, map[string]string, error) {
	// Validate parameters
	absoluteName, ok := parameters["absoluteName"].(string)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for absoluteName")
	}
	linkedRecordName, ok := parameters["linkedRecordName"].(string)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for linkedRecordName")
	}
	properties, ok := parameters["properties"].(map[string]string)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for properties")
	}
	ttl, ok := parameters["ttl"].(int)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for ttl")
	}

	// Define route and parameter map
	route := "/addAliasRecord"
	paramsMap := map[string]string{
		"absoluteName":     absoluteName,
		"linkedRecordName": linkedRecordName,
		"properties":       common.ConvertToSeparatedString(properties, "|"),
		"ttl":              fmt.Sprintf("%d", ttl),
		"viewId":           fmt.Sprintf("%d", viewId),
	}
	return route, paramsMap, nil
}

func prepCreateExternalParams(parameters map[string]interface{}, viewId int) (string, map[string]string, error) {
	// Validate parameters
	name, ok := parameters["name"].(string)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for name")
	}
	properties, ok := parameters["properties"].(map[string]string)
	if !ok {
		return "", nil, fmt.Errorf("invalid type for properties")
	}

	// Define route and parameter map
	route := "/addExternalHostRecord"
	paramsMap := map[string]string{
		"name":       name,
		"properties": common.ConvertToSeparatedString(properties, "|"),
		"viewId":     fmt.Sprintf("%d", viewId),
	}
	return route, paramsMap, nil
}
