package services

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

// GetConfigID returns the BlueCat configuration ID.
//
// Steady state: the ID is supplied via config and cached on the server, so
// this is a pointer dereference. Fallback path hits v2 only when config did
// not supply a configurationId.
func GetConfigID(server interfaces.ServerInterface) (int, error) {
	if id, ok := server.ConfigurationID(); ok {
		return id, nil
	}

	logger.Info("GetConfigID: no cached configurationId, resolving via v2 API")

	resp, err := server.MakeRequest("GET", "/api/v2/configurations", "limit=1", nil)
	if err != nil {
		return 0, fmt.Errorf("resolving configurationId via v2: %w", err)
	}

	var col models.V2Collection[struct {
		ID int `json:"id"`
	}]
	if err := json.Unmarshal(resp, &col); err != nil {
		return 0, fmt.Errorf("decoding /api/v2/configurations response: %w", err)
	}
	if len(col.Data) == 0 {
		return 0, fmt.Errorf("no configurations returned from /api/v2/configurations")
	}

	logger.Info("GetConfigID resolved via v2", zap.Int("configId", col.Data[0].ID))
	return col.Data[0].ID, nil
}

// buildFilter assembles a BlueCat v2 filter querystring value from one or
// more predicates joined by ` and `, returning a fully URL-encoded
// `filter=...` fragment ready to drop into a queryParam string. Pass each
// predicate in its v2 function-call form, e.g. `name:eq('foo')`.
func buildFilter(predicates ...string) string {
	if len(predicates) == 0 {
		return ""
	}
	return "filter=" + url.QueryEscape(strings.Join(predicates, " and "))
}

// --- v1 helpers below — still wired into IpAddressService; Phase 5B retires them ---

// GetParentID retrieves the parent ID of an entity from Bluecat (v1).
func GetParentID(server interfaces.ServerInterface, entityId int) (int, error) {
	logger.Info("GetParentID started", zap.Int("entityId", entityId))

	route, params := "/getParent", fmt.Sprintf("entityId=%d", entityId)
	resp, err := server.MakeRequest("GET", route, params, nil)
	if err != nil {
		logger.Error("Error getting parent ID", zap.Error(err), zap.Int("entityId", entityId))
		return -1, err
	}

	var bluecatEntity models.BluecatEntity
	if err := json.Unmarshal(resp, &bluecatEntity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return -1, err
	}

	if bluecatEntity.IsEmpty() {
		logger.Info("Entity not found", zap.Int("entity id", entityId))
		return -1, &ErrEntityNotFound{}
	}

	parentEntity := bluecatEntity.ToEntity()

	logger.Info("GetParentID successful", zap.Int("parentId", parentEntity.ID))
	return parentEntity.ID, nil
}

// GetEntityByID retrieves an entity by ID from Bluecat (v1).
func GetEntityByID(server interfaces.ServerInterface, id int, includeHA bool, expectedTypes []string) (*models.Entity, error) {
	route, params := "/getEntityById", fmt.Sprintf("id=%d&includeHA=%t", id, includeHA)
	resp, err := server.MakeRequest("GET", route, params, nil)
	if err != nil {
		logger.Error("Error getting entity by ID", zap.Error(err), zap.Int("id", id))
		return nil, err
	}
	logger.Info("Received response for GetEntityByID", zap.ByteString("response", resp))

	var bluecatEntity models.BluecatEntity
	if err := json.Unmarshal(resp, &bluecatEntity); err != nil {
		logger.Error("Error unmarshalling entity response", zap.Error(err))
		return nil, err
	}
	if bluecatEntity.IsEmpty() {
		logger.Info("Entity not found", zap.Int("id", id))
		return nil, &ErrEntityNotFound{}
	}

	entity := bluecatEntity.ToEntity()

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

// ALLOWDELETE — the v1 type allowlist for DeleteEntityByID. Phase 3 retired
// MAC handlers; the MAC entries here are vestigial but harmless. Phase 5B
// removes ALLOWDELETE entirely when DeleteEntityByID retires.
var ALLOWDELETE = []string{
	types.HOSTRECORD,
	types.EXTERNALHOST,
	types.CNAMERECORD,
	types.IP4ADDRESS,
}

// DeleteEntityByID deletes an entity by ID from Bluecat (v1).
func DeleteEntityByID(server interfaces.ServerInterface, id int, expectedTypes []string) error {
	logger.Info("DeleteEntityByID started", zap.Int("id", id))

	entity, err := GetEntityByID(server, id, false, expectedTypes)
	if err != nil {
		return err
	}

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

	route, params := "/delete", fmt.Sprintf("objectId=%d", id)
	_, err = server.MakeRequest("DELETE", route, params, nil)
	if err != nil {
		logger.Error("Error deleting entity", zap.Error(err), zap.Int("id", id))
		return err
	}

	logger.Info("DeleteEntityByID successful", zap.Int("id", id))
	return nil
}
