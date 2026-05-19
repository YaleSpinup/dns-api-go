package services

import (
	"bytes"
	"dns-api-go/internal/common"
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type IpAddressEntityService interface {
	GetIpAddress(address string) (*models.Entity, error)
	DeleteIpAddress(address string) error
	AssignIpAddress(action string, macAddress string, parentId int, hostInfo map[string]string, properties map[string]string) (*models.Entity, error)
	ParentIDFromCIDR(cidr string) (int, error)
}

type IpAddressService struct {
	server interfaces.ServerInterface
}

func NewIpAddressService(server interfaces.ServerInterface) *IpAddressService {
	return &IpAddressService{server: server}
}

// GetIpAddress resolves an IPv4 address string to its v2 Address entity.
// Used by DeleteIpAddress to find the ID; no longer publicly routable
// (Phase 3 removed the GET /ips/{ip} route) but still exposed because
// the api package's parentIdFromCidr fallback may call it.
func (ips *IpAddressService) GetIpAddress(address string) (*models.Entity, error) {
	logger.Info("GetIpAddress started", zap.String("address", address))

	query := buildFilter(fmt.Sprintf("address:eq('%s')", address)) + "&limit=1"
	resp, err := ips.server.MakeRequest("GET", "/api/v2/addresses", query, nil)
	if err != nil {
		return nil, err
	}

	var col models.V2Collection[models.V2Address]
	if err := json.Unmarshal(resp, &col); err != nil {
		return nil, fmt.Errorf("decode addresses lookup for %s: %w", address, err)
	}
	if len(col.Data) == 0 {
		logger.Info("Entity not found", zap.String("ip address", address))
		return nil, &ErrEntityNotFound{}
	}

	entity := col.Data[0].ToEntity()
	return &entity, nil
}

// DeleteIpAddress removes an IPv4 address and any host records that point
// at it, preserving the v1 wire contract (server-api's delete_ip:
// "deletes the IP record … and the associated host records").
//
//  1. Resolve IP → address id via filter lookup.
//  2. List host records pointing at the address via
//     /api/v2/addresses/{id}/resourceRecords and DELETE each.
//  3. DELETE the address itself.
//
// Step 2 surfaces host records as a HAL+JSON collection; if BlueCat ever
// changes the relationship endpoint, the live test in
// ip_address_v2_live_test.go is the canary.
func (ips *IpAddressService) DeleteIpAddress(address string) error {
	logger.Info("DeleteIpAddress started", zap.String("address", address))

	entity, err := ips.GetIpAddress(address)
	if err != nil {
		return err
	}
	addressID := entity.ID

	if err := ips.deleteHostRecordsForAddress(addressID); err != nil {
		return fmt.Errorf("deleting host records for address %s (id=%d): %w", address, addressID, err)
	}

	if _, err := ips.server.MakeRequest("DELETE", fmt.Sprintf("/api/v2/addresses/%d", addressID), "", nil); err != nil {
		if common.IsNotFound(err) {
			return &ErrEntityNotFound{}
		}
		return err
	}

	logger.Info("DeleteIpAddress successful", zap.String("address", address), zap.Int("id", addressID))
	return nil
}

// deleteHostRecordsForAddress walks the address's resourceRecords
// collection and DELETEs each one. Silently skips a 404 on individual
// record deletion (concurrent delete is benign).
func (ips *IpAddressService) deleteHostRecordsForAddress(addressID int) error {
	resp, err := ips.server.MakeRequest("GET", fmt.Sprintf("/api/v2/addresses/%d/resourceRecords", addressID), "limit=100", nil)
	if err != nil {
		// If the address itself is gone, treat as no records to delete.
		if common.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("listing host records for address %d: %w", addressID, err)
	}

	var col models.V2Collection[struct {
		ID int `json:"id"`
	}]
	if err := json.Unmarshal(resp, &col); err != nil {
		return fmt.Errorf("decode host-record list for address %d: %w", addressID, err)
	}

	for _, rec := range col.Data {
		if _, err := ips.server.MakeRequest("DELETE", fmt.Sprintf("/api/v2/resourceRecords/%d", rec.ID), "", nil); err != nil {
			if common.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("deleting host record %d: %w", rec.ID, err)
		}
	}
	return nil
}

// AssignIpAddress allocates the next available IPv4 address in a network
// and creates a HostRecord bound to it, preserving the v1 wire contract
// (server-api's assign_ip: "returns the next available IP in the specified
// CIDR and adds the 'fqdn' host record + ptr record").
//
// v1 was a single RPC; v2 splits the operation:
//  1. POST /api/v2/networks/{parentId}/addresses — allocates IP. The
//     x-bcn-create-reverse-record header drives PTR creation.
//  2. POST /api/v2/zones/{zoneId}/resourceRecords — creates the forward
//     HostRecord bound to the new address by id.
//
// If step 2 fails the just-allocated address is rolled back via DELETE so
// the next attempt doesn't double-allocate.
//
// Returns an Entity shaped like the v1 IP4Address response (ID = address
// id, Properties["address"] = IP string) — the handler builds the final
// `{id, ip}` payload server-api consumes from these two fields.
func (ips *IpAddressService) AssignIpAddress(action string, macAddress string, parentId int, hostInfo map[string]string, properties map[string]string) (*models.Entity, error) {
	logger.Info("AssignIpAddress started",
		zap.String("action", action),
		zap.String("mac", macAddress),
		zap.Int("parentId", parentId))

	hostname := hostInfo["hostname"]
	if hostname == "" {
		return nil, fmt.Errorf("missing required hostInfo.hostname")
	}

	viewIDStr := hostInfo["viewId"]
	viewID, err := strconv.Atoi(viewIDStr)
	if err != nil || viewID == 0 {
		return nil, fmt.Errorf("invalid hostInfo.viewId %q: %w", viewIDStr, err)
	}

	reverseFlag := strings.EqualFold(hostInfo["reverseFlag"], "true")

	addr, err := ips.allocateAddress(parentId, hostname, macAddress, reverseFlag, properties)
	if err != nil {
		return nil, err
	}

	if _, err := ips.createHostRecordForAddress(viewID, hostname, addr, properties); err != nil {
		// Roll back the address allocation so the next attempt sees a
		// clean slate. Best-effort: log and continue if the rollback
		// itself fails.
		if _, rbErr := ips.server.MakeRequest("DELETE", fmt.Sprintf("/api/v2/addresses/%d", addr.ID), "", nil); rbErr != nil {
			logger.Error("rollback of allocated address failed",
				zap.Int("addressId", addr.ID),
				zap.Error(rbErr))
		}
		return nil, fmt.Errorf("creating host record for %s: %w", hostname, err)
	}

	entity := addr.ToEntity()
	logger.Info("AssignIpAddress successful",
		zap.Int("addressId", addr.ID),
		zap.String("address", addr.Address))
	return &entity, nil
}

// allocateAddress POSTs a state=STATIC IPv4Address into the network. With
// no `address` field, BlueCat picks the next available — the v1
// /assignNextAvailableIP4Address semantic. User-defined fields from
// server-api's `properties` map (e.g. Yale's required `phone`) get nested
// under `userDefinedFields` per the v2 schema.
func (ips *IpAddressService) allocateAddress(parentId int, hostname, macAddress string, reverseFlag bool, properties map[string]string) (*models.V2Address, error) {
	body := map[string]interface{}{
		"type":  "IPv4Address",
		"state": "STATIC",
	}
	if hostname != "" {
		body["name"] = hostname
	}
	if macAddress != "" {
		body["macAddress"] = map[string]string{"address": macAddress}
	}
	if udfs := userDefinedFieldsFromProperties(properties); len(udfs) > 0 {
		body["userDefinedFields"] = udfs
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal address body: %w", err)
	}

	queryParam := ""
	if reverseFlag {
		queryParam = "x-bcn-create-reverse-record=true"
	}

	resp, err := ips.server.MakeRequest("POST", fmt.Sprintf("/api/v2/networks/%d/addresses", parentId), queryParam, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("POST address: %w", err)
	}

	var addr models.V2Address
	if err := json.Unmarshal(resp, &addr); err != nil {
		return nil, fmt.Errorf("decode address response: %w", err)
	}
	if addr.ID == 0 || addr.Address == "" {
		return nil, fmt.Errorf("address response missing id/address: %s", string(resp))
	}
	return &addr, nil
}

// createHostRecordForAddress creates a HostRecord under the zone derived
// from `fqdn`, bound to the supplied address by id. UDFs from the
// AssignIpAddress caller's properties map land on the HostRecord too —
// BAMs that gate Address allocation on a required UDF generally gate
// HostRecord creation the same way.
func (ips *IpAddressService) createHostRecordForAddress(viewID int, fqdn string, addr *models.V2Address, properties map[string]string) (*models.V2HostRecord, error) {
	zoneID, localName, err := splitFQDN(ips.server, fqdn, viewID)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"type": "HostRecord",
		"name": localName,
		"addresses": []map[string]interface{}{
			{"id": addr.ID, "type": addr.Type},
		},
	}
	if udfs := userDefinedFieldsFromProperties(properties); len(udfs) > 0 {
		body["userDefinedFields"] = udfs
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal host record body: %w", err)
	}

	resp, err := ips.server.MakeRequest("POST", fmt.Sprintf("/api/v2/zones/%d/resourceRecords", zoneID), "", bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	var rec models.V2HostRecord
	if err := json.Unmarshal(resp, &rec); err != nil {
		return nil, fmt.Errorf("decode host record create response: %w", err)
	}
	return &rec, nil
}

// ParentIDFromCIDR resolves a CIDR string ("10.5.0.0/24") to the v2
// network resource ID that owns it. Replaces the v1 probe-walk in the
// api package: enumerate first 10 IPs, look up each, walk getParent.
//
// v2 lets us query networks directly by their CIDR range. The OpenAPI
// schema's Network.range field carries the CIDR, and filter predicate
// `range:eq('{cidr}')` is supported on /api/v2/networks per the v2
// filter grammar. The plan flagged this as the one v2 simplification
// pending live validation; ip_address_v2_live_test.go is the canary.
func (ips *IpAddressService) ParentIDFromCIDR(cidr string) (int, error) {
	if cidr == "" {
		return 0, fmt.Errorf("CIDR cannot be empty")
	}

	query := buildFilter(fmt.Sprintf("range:eq('%s')", cidr)) + "&limit=1"
	resp, err := ips.server.MakeRequest("GET", "/api/v2/networks", query, nil)
	if err != nil {
		return 0, fmt.Errorf("looking up network by CIDR %s: %w", cidr, err)
	}

	var col models.V2Collection[struct {
		ID int `json:"id"`
	}]
	if err := json.Unmarshal(resp, &col); err != nil {
		return 0, fmt.Errorf("decode network lookup for %s: %w", cidr, err)
	}
	if len(col.Data) == 0 {
		return 0, fmt.Errorf("no network found for CIDR %s", cidr)
	}
	return col.Data[0].ID, nil
}
