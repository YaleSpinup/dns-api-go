package services

import (
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

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

// coreFieldKeys are the keys that v1 callers historically stuffed into the
// pipe-delimited `properties` string but that v2 surfaces as top-level
// fields on Address / HostRecord bodies. They are NOT user-defined fields
// and must not go into `userDefinedFields` — the callers already pull
// them out (or simply don't use them) before building the request body.
var coreFieldKeys = map[string]struct{}{
	"":                 {},
	"type":             {},
	"state":            {},
	"address":          {},
	"addresses":        {},
	"name":             {},
	"absoluteName":     {},
	"macAddress":       {},
	"reverseRecord":    {},
	"linkedRecord":     {},
	"linkedRecordName": {},
	"ttl":              {},
}

// defaultAddressUDFs returns the user-defined field set Yale's BAM
// requires when allocating a v2 IPv4Address, used as a fallback when the
// caller supplied no UDFs of its own.
//
// The set mirrors the literal that server-api hardcodes for its
// `assign_ip` flow (see server-api/lib/actions/server/base.rb:899-900):
//
//	machine_type=Virtual machine
//	description=Auto-provisioned by Spinup ServerAPI
//	phone=xxx
//	location=Cloud
//	reg_by=SpinupManaged
//	reg_date=<UTC YYYY-MM-DD HH:MM:SS>
//	user_name=<requesting user>
//
// server-api's `create_host_record` flow doesn't pass properties at all,
// so a v2 auto-allocation from CreateRecord would otherwise reach BAM
// with no `userDefinedFields` and get rejected one required field at a
// time. Under v1 BAM silently auto-created Addresses during
// addHostRecord without validating UDFs; v2 splits the operation and
// validates strictly, so dns-api-go fills the gap here.
//
// `reg_date` is computed at call time using the same format
// (`%Y-%m-%d %H:%M:%S` UTC) as server-api's `time_proteus` helper.
// `user_name` falls back to a service identifier — dns-api-go has no
// upstream user context on the create_host_record path. Making this
// config-driven (so the value set can shift without a code change) is
// the proper follow-up.
func defaultAddressUDFs() map[string]interface{} {
	return map[string]interface{}{
		"machine_type": "Virtual machine",
		"description":  "Auto-provisioned by Spinup ServerAPI",
		"phone":        "xxx",
		"location":     "Cloud",
		"reg_by":       "Spinup",
		"reg_date":     time.Now().UTC().Format("2006-01-02 15:04:05"),
		"user_name":    "spinup-dns-api",
	}
}

// userDefinedFieldsFromProperties extracts user-defined fields from the
// v1-shaped properties map (key=value pairs that the handlers parse from a
// pipe-delimited string). v2 requires UDFs to live under the
// `userDefinedFields` object on resource bodies, not at the top level —
// Yale's BAM, for example, makes `phone` a required UDF on IPv4Address
// and rejects allocation POSTs that omit it.
//
// Returns nil when there are no UDFs to send (the caller then omits the
// `userDefinedFields` key entirely rather than sending an empty object).
func userDefinedFieldsFromProperties(properties map[string]string) map[string]interface{} {
	if len(properties) == 0 {
		return nil
	}
	udfs := make(map[string]interface{}, len(properties))
	for k, v := range properties {
		if _, isCore := coreFieldKeys[k]; isCore {
			continue
		}
		if v == "" {
			continue
		}
		udfs[k] = v
	}
	if len(udfs) == 0 {
		return nil
	}
	return udfs
}

// splitFQDN separates an absolute name into the local record label and the
// zone ID it should live under. e.g. "host.spinuptest.internal" with view
// 100902 returns (100913, "host", nil) where 100913 is the spinuptest zone.
// Both RecordService.CreateRecord and IpAddressService.AssignIpAddress
// need this on their create paths.
func splitFQDN(server interfaces.ServerInterface, absoluteName string, viewId int) (int, string, error) {
	if absoluteName == "" {
		return 0, "", fmt.Errorf("empty absoluteName")
	}
	parts := strings.Split(absoluteName, ".")
	if len(parts) < 2 {
		return 0, "", fmt.Errorf("absoluteName %q has no zone component", absoluteName)
	}
	zoneID, err := resolveZoneIDFromLabels(server, parts[1:], viewId)
	if err != nil {
		return 0, "", err
	}
	return zoneID, parts[0], nil
}

// resolveZoneIDFromLabels walks BlueCat's zone tree right-to-left,
// descending from the view into nested zones until every label is matched.
// "spinuptest.internal" with view 100902 → first matches Zone(name='internal')
// under views/100902/zones, then Zone(name='spinuptest') under zones/{id}/zones.
//
// At the view level, type:eq('Zone') disambiguates between Zone and
// ExternalHostsZone (which can share a name — Phase 1 finding). At deeper
// levels the type filter is rejected by BAM with HTTP 400
// InvalidFilterField — sub-zones of a Zone can only be type=Zone anyway,
// so dropping the predicate is both safe and required.
func resolveZoneIDFromLabels(server interfaces.ServerInterface, zoneLabels []string, viewId int) (int, error) {
	if len(zoneLabels) == 0 {
		return 0, fmt.Errorf("no zone labels to resolve")
	}
	collectionRoute := fmt.Sprintf("/api/v2/views/%d/zones", viewId)
	atViewLevel := true
	var zoneID int
	for i := len(zoneLabels) - 1; i >= 0; i-- {
		label := zoneLabels[i]
		predicates := []string{fmt.Sprintf("name:eq('%s')", label)}
		if atViewLevel {
			predicates = append(predicates, "type:eq('Zone')")
		}
		query := buildFilter(predicates...) + "&limit=1"
		resp, err := server.MakeRequest("GET", collectionRoute, query, nil)
		if err != nil {
			return 0, fmt.Errorf("looking up zone %q under %s: %w", label, collectionRoute, err)
		}
		var col models.V2Collection[struct {
			ID int `json:"id"`
		}]
		if err := json.Unmarshal(resp, &col); err != nil {
			return 0, fmt.Errorf("decode zone lookup for %q: %w", label, err)
		}
		if len(col.Data) == 0 {
			return 0, fmt.Errorf("zone %q not found under %s", label, collectionRoute)
		}
		zoneID = col.Data[0].ID
		collectionRoute = fmt.Sprintf("/api/v2/zones/%d/zones", zoneID)
		atViewLevel = false
	}
	return zoneID, nil
}
