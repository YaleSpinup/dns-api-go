package models

import (
	"dns-api-go/internal/common"
	"encoding/json"
)

type Entity struct {
	ID         int               `json:"id" example:"12345"`
	Name       string            `json:"name" exampple:"server-001.yale.edu"`
	Type       string            `json:"type" example:"HostRecord"`
	Properties map[string]string `json:"properties" example:"address:10.0.1.15|ttl:3600"`
}

// ToBluecatJSON Converts an Entity to a JSON byte array for Bluecat
func (e Entity) ToBluecatJSON() ([]byte, error) {
	bluecatEntity := BluecatEntity{
		ID:         e.ID,
		Name:       &e.Name,
		Type:       &e.Type,
		Properties: common.StringPtr(common.ConvertToSeparatedString(e.Properties, "|")),
	}
	return json.Marshal(bluecatEntity)
}
