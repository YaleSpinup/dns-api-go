package models

import (
	"dns-api-go/internal/common"
	"encoding/json"
)

type Entity struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
}

// ToBluecatJSON Converts an Entity to a JSON byte array for Bluecat
func (e Entity) ToBluecatJSON() ([]byte, error) {
	type BluecatEntity struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Type       string `json:"type"`
		Properties string `json:"properties"`
	}
	bluecatEntity := BluecatEntity{
		ID:         e.ID,
		Name:       e.Name,
		Type:       e.Type,
		Properties: common.ConvertToSeparatedString(e.Properties, "|"),
	}
	return json.Marshal(bluecatEntity)
}
