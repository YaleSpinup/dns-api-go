package services

import (
	"dns-api-go/internal/interfaces"
	"dns-api-go/internal/models"
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
