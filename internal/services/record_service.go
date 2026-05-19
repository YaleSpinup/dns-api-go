package services

import (
	"bytes"
	"dns-api-go/internal/common"
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/internal/types"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"strings"

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

// recordTypes are the resource-record kinds dns-api-go exposes. The
// EntityGetter/Deleter interfaces accept opaque IDs, so we still gate by
// type so that callers cannot use record endpoints to mutate other v2
// resources.
var recordTypes = []string{types.HOSTRECORD, types.CNAMERECORD, types.EXTERNALHOST}

func NewRecordService(server interfaces.ServerInterface) *RecordService {
	return &RecordService{server: server}
}

// GetEntity fetches a single resource record by ID via v2. includeHA is
// accepted for backward compatibility with the EntityGetter interface but
// has no v2 equivalent (v2 always returns the full entity).
func (rs *RecordService) GetEntity(recordId int, _ bool) (*models.Entity, error) {
	logger.Info("RecordService GetEntity started", zap.Int("recordId", recordId))

	resp, err := rs.server.MakeRequest("GET", fmt.Sprintf("/api/v2/resourceRecords/%d", recordId), "", nil)
	if err != nil {
		if common.IsNotFound(err) {
			return nil, &ErrEntityNotFound{}
		}
		return nil, err
	}

	var rec models.V2HostRecord
	if err := json.Unmarshal(resp, &rec); err != nil {
		return nil, fmt.Errorf("decode resourceRecord %d: %w", recordId, err)
	}

	if !common.Contains(recordTypes, rec.Type) {
		return nil, &ErrEntityTypeMismatch{ExpectedTypes: recordTypes, ActualType: rec.Type}
	}

	entity := rec.ToEntity()
	logger.Info("GetEntity successful",
		zap.Int("entityId", entity.ID),
		zap.String("entityType", entity.Type))
	return &entity, nil
}

// DeleteEntity deletes a resource record by ID. The record type is checked
// first to preserve the v1 type-allowlist (record handlers must not be a
// path to delete arbitrary v2 resources).
func (rs *RecordService) DeleteEntity(recordId int) error {
	logger.Info("RecordService DeleteEntity started", zap.Int("recordId", recordId))

	if _, err := rs.GetEntity(recordId, false); err != nil {
		return err
	}

	_, err := rs.server.MakeRequest("DELETE", fmt.Sprintf("/api/v2/resourceRecords/%d", recordId), "", nil)
	if err != nil {
		if common.IsNotFound(err) {
			return &ErrEntityNotFound{}
		}
		return err
	}

	logger.Info("DeleteEntity successful", zap.Int("recordId", recordId))
	return nil
}

// GetRecordsByType searches resource records of one of the supported kinds.
// The v2 implementation funnels every record kind through
// /api/v2/resourceRecords?filter=… — there is no per-type collection
// endpoint, and absoluteName covers the search surface that v1 split across
// hint, name, and keyword variants.
//
// Parameters honored (all optional, sourced from the GET /records handler):
//   - options.hint:   absoluteName contains
//   - name:           absoluteName equals (exact match)
//   - keyword:        absoluteName contains (treated as a hint synonym)
//   - count, start:   limit/offset
//
// viewId is kept in the signature for backward compatibility but is not
// applied as a v2 filter — v2 resourceRecords are globally addressable and
// the view is implicit in the zone hierarchy.
func (rs *RecordService) GetRecordsByType(recordType string, parameters map[string]interface{}, _ int) (*[]models.Entity, error) {
	logger.Info("RecordService GetRecordsByType started", zap.String("recordType", recordType))

	if !common.Contains(recordTypes, recordType) {
		return nil, fmt.Errorf("invalid record type %q", recordType)
	}

	count, ok := parameters["count"].(int)
	if !ok {
		return nil, fmt.Errorf("invalid type for count")
	}
	start, ok := parameters["start"].(int)
	if !ok {
		return nil, fmt.Errorf("invalid type for start")
	}

	predicates := []string{fmt.Sprintf("type:eq('%s')", recordType)}
	if p := extractAbsoluteNamePredicate(parameters); p != "" {
		predicates = append(predicates, p)
	}

	query := buildFilter(predicates...)
	if count > 0 {
		query += fmt.Sprintf("&limit=%d", count)
	}
	if start > 0 {
		query += fmt.Sprintf("&offset=%d", start)
	}

	resp, err := rs.server.MakeRequest("GET", "/api/v2/resourceRecords", query, nil)
	if err != nil {
		return nil, err
	}

	var col models.V2Collection[models.V2HostRecord]
	if err := json.Unmarshal(resp, &col); err != nil {
		return nil, fmt.Errorf("decode resourceRecord collection: %w", err)
	}

	entities := make([]models.Entity, 0, len(col.Data))
	for _, rec := range col.Data {
		entities = append(entities, rec.ToEntity())
	}
	return &entities, nil
}

// extractAbsoluteNamePredicate maps the handler's name/hint/keyword params
// to a single v2 filter predicate against absoluteName. Empty string means
// no name-based predicate (the caller will still apply type:eq).
func extractAbsoluteNamePredicate(parameters map[string]interface{}) string {
	if name, _ := parameters["name"].(string); name != "" {
		return fmt.Sprintf("absoluteName:eq('%s')", name)
	}
	if hint, _ := optionsHint(parameters); hint != "" {
		return fmt.Sprintf("absoluteName:contains('%s')", hint)
	}
	if kw, _ := parameters["keyword"].(string); kw != "" {
		return fmt.Sprintf("absoluteName:contains('%s')", kw)
	}
	return ""
}

func optionsHint(parameters map[string]interface{}) (string, bool) {
	opts, ok := parameters["options"].(map[string]string)
	if !ok {
		return "", false
	}
	h := opts["hint"]
	return h, h != ""
}

// CreateRecord creates a HostRecord, AliasRecord, or ExternalHostRecord via
// v2. The wire contract with server-api is preserved end-to-end: callers
// pass the FQDN as absoluteName / target name; this layer resolves the
// zone, address IDs (auto-allocating missing v2 Address resources when
// server-api passes a target IP that BAM hasn't seen yet — v1
// `addHostRecord` did this implicitly, v2 requires the explicit POST), and
// the discriminated body shape.
func (rs *RecordService) CreateRecord(recordType string, parameters map[string]interface{}, viewId int) (*models.Entity, error) {
	logger.Info("Create Record started", zap.String("recordType", recordType))

	if !common.Contains(recordTypes, recordType) {
		return nil, fmt.Errorf("invalid record type %q", recordType)
	}

	// Preserve v1's pre-flight existence check so callers continue to see
	// 409 Conflict instead of whatever shape BlueCat returns on duplicate.
	if existing, found, err := rs.findByAbsoluteName(recordType, parameters); err == nil && found {
		logger.Error("Record already exists", zap.String("recordType", recordType))
		return nil, &ErrEntityAlreadyExists{EntityID: existing.Name}
	}

	body, zoneID, allocated, err := rs.buildCreateBody(recordType, parameters, viewId)
	if err != nil {
		// Allocation may have partially succeeded before the failure (e.g.
		// 2 of 3 IPs allocated, 3rd network lookup failed). Roll back what
		// we have so retries see a clean slate.
		rs.rollbackAddresses(allocated)
		return nil, err
	}

	resp, err := rs.server.MakeRequest(
		"POST",
		fmt.Sprintf("/api/v2/zones/%d/resourceRecords", zoneID),
		"",
		bytes.NewReader(body),
	)
	if err != nil {
		rs.rollbackAddresses(allocated)
		return nil, err
	}

	var rec models.V2HostRecord
	if err := json.Unmarshal(resp, &rec); err != nil {
		rs.rollbackAddresses(allocated)
		return nil, fmt.Errorf("decode create-record response: %w", err)
	}

	entity := rec.ToEntity()
	return &entity, nil
}

// rollbackAddresses DELETEs each v2 Address by ID. Best-effort: failures are
// logged but don't surface, since the parent operation is already failing
// and we want to drop as much partial state as possible.
func (rs *RecordService) rollbackAddresses(ids []int) {
	for _, id := range ids {
		if _, err := rs.server.MakeRequest("DELETE", fmt.Sprintf("/api/v2/addresses/%d", id), "", nil); err != nil {
			logger.Error("rollback of newly-allocated address failed",
				zap.Int("addressId", id),
				zap.Error(err))
		}
	}
}

// findByAbsoluteName runs a v2 search filtered by absoluteName + type and
// returns the first match, or (_, false, nil) if no record matches.
func (rs *RecordService) findByAbsoluteName(recordType string, parameters map[string]interface{}) (*models.Entity, bool, error) {
	var fqdn string
	if recordType == types.EXTERNALHOST {
		fqdn, _ = parameters["name"].(string)
	} else {
		fqdn, _ = parameters["absoluteName"].(string)
	}
	if fqdn == "" {
		return nil, false, nil
	}

	query := buildFilter(
		fmt.Sprintf("type:eq('%s')", recordType),
		fmt.Sprintf("absoluteName:eq('%s')", fqdn),
	) + "&limit=1"

	resp, err := rs.server.MakeRequest("GET", "/api/v2/resourceRecords", query, nil)
	if err != nil {
		return nil, false, err
	}

	var col models.V2Collection[models.V2HostRecord]
	if err := json.Unmarshal(resp, &col); err != nil {
		return nil, false, fmt.Errorf("decode pre-create lookup: %w", err)
	}
	if len(col.Data) == 0 {
		return nil, false, nil
	}
	e := col.Data[0].ToEntity()
	return &e, true, nil
}

// buildCreateBody returns the JSON body, resolved zone ID, and the list of
// v2 Address IDs that were freshly allocated as part of preparing the body
// (HostRecord only — Alias and ExternalHost return nil). The caller must
// roll those IDs back if the subsequent POST fails.
func (rs *RecordService) buildCreateBody(recordType string, parameters map[string]interface{}, viewId int) ([]byte, int, []int, error) {
	switch recordType {
	case types.HOSTRECORD:
		return rs.buildHostRecordBody(parameters, viewId)
	case types.CNAMERECORD:
		body, zoneID, err := rs.buildAliasRecordBody(parameters, viewId)
		return body, zoneID, nil, err
	case types.EXTERNALHOST:
		body, zoneID, err := rs.buildExternalHostRecordBody(parameters, viewId)
		return body, zoneID, nil, err
	default:
		return nil, 0, nil, fmt.Errorf("invalid record type %q", recordType)
	}
}

type v2AddressRef struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
}

func (rs *RecordService) buildHostRecordBody(parameters map[string]interface{}, viewId int) ([]byte, int, []int, error) {
	absoluteName, _ := parameters["absoluteName"].(string)
	if absoluteName == "" {
		return nil, 0, nil, fmt.Errorf("missing absoluteName for HostRecord")
	}
	ips, ok := parameters["addresses"].([]string)
	if !ok || len(ips) == 0 {
		return nil, 0, nil, fmt.Errorf("missing addresses for HostRecord")
	}
	ttl, _ := parameters["ttl"].(int)

	zoneID, localName, err := splitFQDN(rs.server, absoluteName, viewId)
	if err != nil {
		return nil, 0, nil, err
	}

	addrRefs, allocated, err := rs.resolveOrAllocateAddresses(ips)
	if err != nil {
		// Partial allocations get rolled back by the caller via the
		// returned slice — even on the error path.
		return nil, 0, allocated, err
	}

	body := map[string]interface{}{
		"type":      types.HOSTRECORD,
		"name":      localName,
		"addresses": addrRefs,
	}
	if ttl > 0 {
		body["ttl"] = ttl
	}
	if reverse, ok := reverseRecordFromProperties(parameters); ok {
		body["reverseRecord"] = reverse
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, 0, allocated, fmt.Errorf("marshal HostRecord body: %w", err)
	}
	return encoded, zoneID, allocated, nil
}

func (rs *RecordService) buildAliasRecordBody(parameters map[string]interface{}, viewId int) ([]byte, int, error) {
	absoluteName, _ := parameters["absoluteName"].(string)
	if absoluteName == "" {
		return nil, 0, fmt.Errorf("missing absoluteName for AliasRecord")
	}
	target, _ := parameters["linkedRecordName"].(string)
	if target == "" {
		return nil, 0, fmt.Errorf("missing linkedRecordName for AliasRecord")
	}
	ttl, _ := parameters["ttl"].(int)

	zoneID, localName, err := splitFQDN(rs.server, absoluteName, viewId)
	if err != nil {
		return nil, 0, err
	}

	body := map[string]interface{}{
		"type": types.CNAMERECORD,
		"name": localName,
		"linkedRecord": map[string]string{
			"absoluteName": target,
		},
	}
	if ttl > 0 {
		body["ttl"] = ttl
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal AliasRecord body: %w", err)
	}
	return encoded, zoneID, nil
}

func (rs *RecordService) buildExternalHostRecordBody(parameters map[string]interface{}, viewId int) ([]byte, int, error) {
	name, _ := parameters["name"].(string)
	if name == "" {
		return nil, 0, fmt.Errorf("missing name for ExternalHostRecord")
	}

	zoneID, err := rs.resolveExternalHostsZone(viewId)
	if err != nil {
		return nil, 0, err
	}

	body := map[string]interface{}{
		"type": types.EXTERNALHOST,
		"name": name,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, 0, fmt.Errorf("marshal ExternalHostRecord body: %w", err)
	}
	return encoded, zoneID, nil
}

// reverseRecordFromProperties pulls the optional "reverseRecord" boolean
// out of the v1-shaped properties map ("true"/"false" or "1"/"0").
func reverseRecordFromProperties(parameters map[string]interface{}) (bool, bool) {
	props, ok := parameters["properties"].(map[string]string)
	if !ok {
		return false, false
	}
	v, ok := props["reverseRecord"]
	if !ok {
		return false, false
	}
	switch strings.ToLower(v) {
	case "true", "1", "yes":
		return true, true
	case "false", "0", "no":
		return false, true
	}
	return false, false
}

// resolveExternalHostsZone returns the single ExternalHostsZone under the
// given view (Yale's views have exactly one).
func (rs *RecordService) resolveExternalHostsZone(viewId int) (int, error) {
	query := buildFilter("type:eq('ExternalHostsZone')") + "&limit=1"
	resp, err := rs.server.MakeRequest("GET", fmt.Sprintf("/api/v2/views/%d/zones", viewId), query, nil)
	if err != nil {
		return 0, fmt.Errorf("looking up ExternalHostsZone for view %d: %w", viewId, err)
	}
	var col models.V2Collection[struct {
		ID int `json:"id"`
	}]
	if err := json.Unmarshal(resp, &col); err != nil {
		return 0, fmt.Errorf("decode ExternalHostsZone lookup: %w", err)
	}
	if len(col.Data) == 0 {
		return 0, fmt.Errorf("no ExternalHostsZone found under view %d", viewId)
	}
	return col.Data[0].ID, nil
}

// resolveOrAllocateAddresses maps each IP string to a v2 Address resource
// reference, allocating one when BAM doesn't yet have the IP as an
// Address. v2 HostRecord create only accepts addresses by {id, type}; v1's
// addHostRecord auto-created Addresses for any IPs in `target` that didn't
// exist, and server-api's create_host_record flow still depends on that
// behavior (no separate POST /ips beforehand).
//
// Returns the refs and the list of Address IDs that were freshly allocated
// (vs. found pre-existing). The caller rolls those back if the subsequent
// HostRecord POST fails. Returning the allocated list even on the error
// path is intentional — if the 3rd of 4 IPs fails to allocate, the first 2
// allocations need to be undone.
func (rs *RecordService) resolveOrAllocateAddresses(ips []string) ([]v2AddressRef, []int, error) {
	refs := make([]v2AddressRef, 0, len(ips))
	var allocated []int
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}

		existing, found, err := rs.findAddressByIP(ip)
		if err != nil {
			return refs, allocated, err
		}
		if found {
			refs = append(refs, v2AddressRef{ID: existing.ID, Type: existing.Type})
			continue
		}

		netID, err := rs.networkIDContainingIP(ip)
		if err != nil {
			return refs, allocated, fmt.Errorf("locating network for %s: %w", ip, err)
		}

		created, err := rs.allocateAddressInNetwork(netID, ip)
		if err != nil {
			return refs, allocated, fmt.Errorf("allocating address %s: %w", ip, err)
		}
		refs = append(refs, v2AddressRef{ID: created.ID, Type: created.Type})
		allocated = append(allocated, created.ID)
	}
	if len(refs) == 0 {
		return nil, allocated, fmt.Errorf("no usable IP addresses supplied")
	}
	return refs, allocated, nil
}

// findAddressByIP looks up a v2 Address by its IP string. Returns
// (_, false, nil) when the IP isn't in BAM yet.
func (rs *RecordService) findAddressByIP(ip string) (*models.V2Address, bool, error) {
	query := buildFilter(fmt.Sprintf("address:eq('%s')", ip)) + "&limit=1"
	resp, err := rs.server.MakeRequest("GET", "/api/v2/addresses", query, nil)
	if err != nil {
		return nil, false, fmt.Errorf("looking up address %s: %w", ip, err)
	}
	var col models.V2Collection[models.V2Address]
	if err := json.Unmarshal(resp, &col); err != nil {
		return nil, false, fmt.Errorf("decode address %s lookup: %w", ip, err)
	}
	if len(col.Data) == 0 {
		return nil, false, nil
	}
	return &col.Data[0], true, nil
}

// networkIDContainingIP returns the ID of the v2 IPv4Network whose `range`
// CIDR contains the given IP. Uses BAM's `range:contains('<ip>')` filter —
// validated live against BAM-test; the variant `range:contains('<ip>/32')`
// returns 400 InvalidFilterAddress, so the IP must be bare.
func (rs *RecordService) networkIDContainingIP(ip string) (int, error) {
	query := buildFilter(fmt.Sprintf("range:contains('%s')", ip)) + "&limit=1"
	resp, err := rs.server.MakeRequest("GET", "/api/v2/networks", query, nil)
	if err != nil {
		return 0, fmt.Errorf("network lookup by IP %s: %w", ip, err)
	}
	var col models.V2Collection[struct {
		ID int `json:"id"`
	}]
	if err := json.Unmarshal(resp, &col); err != nil {
		return 0, fmt.Errorf("decode network lookup for %s: %w", ip, err)
	}
	if len(col.Data) == 0 {
		return 0, fmt.Errorf("no network contains %s", ip)
	}
	return col.Data[0].ID, nil
}

// allocateAddressInNetwork POSTs a state=STATIC IPv4Address with an
// explicit `address` field — telling BAM to reserve that specific IP
// (vs. the "next available" semantic used by IpAddressService).
func (rs *RecordService) allocateAddressInNetwork(netID int, ip string) (*models.V2Address, error) {
	body := map[string]interface{}{
		"type":    "IPv4Address",
		"state":   "STATIC",
		"address": ip,
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal allocate body: %w", err)
	}
	resp, err := rs.server.MakeRequest(
		"POST",
		fmt.Sprintf("/api/v2/networks/%d/addresses", netID),
		"",
		bytes.NewReader(encoded),
	)
	if err != nil {
		return nil, err
	}
	var addr models.V2Address
	if err := json.Unmarshal(resp, &addr); err != nil {
		return nil, fmt.Errorf("decode allocate response: %w", err)
	}
	if addr.ID == 0 || addr.Address == "" {
		return nil, fmt.Errorf("allocate returned bad address: %s", string(resp))
	}
	return &addr, nil
}

