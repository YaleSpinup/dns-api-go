package services

import (
	"dns-api-go/internal/bluecat"
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
	// V2: GET /api/v2/resourceRecords?filter=type:eq('{type}') and name:contains('{hint}')
	if recordType != types.HOSTRECORD && recordType != types.CNAMERECORD {
		return nil, fmt.Errorf("invalid record type")
	}

	route := "/api/v2/resourceRecords"
	params := fmt.Sprintf("limit=%d&offset=%d", count, start)

	// Build filter with record type and hint
	var filterParts []string
	filterParts = append(filterParts, fmt.Sprintf("type:eq('%s')", recordType))
	if hint, ok := options["hint"]; ok && hint != "" {
		filterParts = append(filterParts, fmt.Sprintf("name:contains('%s')", hint))
	}
	params += "&filter=" + url.QueryEscape(strings.Join(filterParts, " and "))

	resp, err := rs.server.MakeRequest("GET", route, params, nil)
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

	// Check if record already exists in bluecat
	checkRecordParams := map[string]interface{}{
		"name":    parameters["name"],
		"count":   10,
		"start":   0,
		"options": map[string]string{"hint": parameters["name"].(string)},
	}
	// Check if any entities are returned from the search. If there are, check if the first entity's name
	// matches the name of the record being created.
	entities, err := rs.GetRecordsByType(recordType, checkRecordParams, viewId)
	logger.Info("Entities", zap.Any("entities", entities))
	if err == nil && len(*entities) > 0 {
		entity := (*entities)[0]
		// For host/alias records, the absolute name must be retrieved from the properties field of the first entity
		// and checked to see if it matches the "name" parameter that is passed in.
		if recordType == types.HOSTRECORD || recordType == types.CNAMERECORD {
			absoluteName, ok := entity.Properties["absoluteName"]
			if ok && absoluteName == parameters["name"].(string) {
				// Entity already exists, return custom error
				logger.Error("Record already exists", zap.String("recordType", recordType))
				return nil, &ErrEntityAlreadyExists{EntityID: entity.Name}
			}
		} else {
			// For external records, the name parameter is the name of the record, so we can directly compare it with the entity name.
			if entity.Name == parameters["name"].(string) {
				// Entity already exists, return custom error
				logger.Error("Record already exists", zap.String("recordType", recordType))
				return nil, &ErrEntityAlreadyExists{EntityID: entity.Name}
			}
		}
	}

	// Build V2 JSON body based on record type
	var v2Body map[string]interface{}
	switch recordType {
	case types.HOSTRECORD:
		v2Body, err = prepCreateHostBody(parameters, viewId)
		if err != nil {
			return nil, err
		}
	case types.CNAMERECORD:
		v2Body, err = prepCreateCNAMEBody(parameters, viewId)
		if err != nil {
			return nil, err
		}
	case types.EXTERNALHOST:
		v2Body, err = prepCreateExternalBody(parameters, viewId)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid record type")
	}

	// V2: POST /api/v2/resourceRecords with JSON body
	route := "/api/v2/resourceRecords"
	bodyJSON, err := json.Marshal(v2Body)
	if err != nil {
		logger.Error("Error marshalling record request", zap.Error(err))
		return nil, err
	}

	body := strings.NewReader(string(bodyJSON))
	resp, err := rs.server.MakeRequest("POST", route, "", body)
	if err != nil {
		logger.Info("Error code", zap.Error(err))
		return nil, err
	}

	// Unmarshal V2 entity response
	var v2Entity bluecat.V2Entity
	if err := json.Unmarshal(resp, &v2Entity); err != nil {
		logger.Error("Error unmarshalling record response", zap.Error(err))
		return nil, err
	}

	// Get the new entity details
	entity, err := rs.GetEntity(v2Entity.ID, true)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func prepCreateHostBody(parameters map[string]interface{}, viewId int) (map[string]interface{}, error) {
	absoluteName, ok := parameters["absoluteName"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid type for absoluteName")
	}
	addresses, ok := parameters["addresses"].([]string)
	if !ok {
		return nil, fmt.Errorf("invalid type for addresses")
	}
	properties, ok := parameters["properties"].(map[string]string)
	if !ok {
		return nil, fmt.Errorf("invalid type for properties")
	}
	ttl, ok := parameters["ttl"].(int)
	if !ok {
		return nil, fmt.Errorf("invalid type for ttl")
	}

	return map[string]interface{}{
		"type":         types.HOSTRECORD,
		"absoluteName": absoluteName,
		"addresses":    addresses,
		"properties":   properties,
		"ttl":          ttl,
		"viewId":       viewId,
	}, nil
}

func prepCreateCNAMEBody(parameters map[string]interface{}, viewId int) (map[string]interface{}, error) {
	absoluteName, ok := parameters["absoluteName"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid type for absoluteName")
	}
	linkedRecordName, ok := parameters["linkedRecordName"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid type for linkedRecordName")
	}
	properties, ok := parameters["properties"].(map[string]string)
	if !ok {
		return nil, fmt.Errorf("invalid type for properties")
	}
	ttl, ok := parameters["ttl"].(int)
	if !ok {
		return nil, fmt.Errorf("invalid type for ttl")
	}

	return map[string]interface{}{
		"type":             types.CNAMERECORD,
		"absoluteName":     absoluteName,
		"linkedRecordName": linkedRecordName,
		"properties":       properties,
		"ttl":              ttl,
		"viewId":           viewId,
	}, nil
}

func prepCreateExternalBody(parameters map[string]interface{}, viewId int) (map[string]interface{}, error) {
	name, ok := parameters["name"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid type for name")
	}
	properties, ok := parameters["properties"].(map[string]string)
	if !ok {
		return nil, fmt.Errorf("invalid type for properties")
	}

	return map[string]interface{}{
		"type":       types.EXTERNALHOST,
		"name":       name,
		"properties": properties,
		"viewId":     viewId,
	}, nil
}
