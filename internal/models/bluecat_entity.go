package models

import "dns-api-go/internal/common"

type BluecatEntity struct {
	ID         int     `json:"id"`
	Name       *string `json:"name"`
	Type       *string `json:"type"`
	Properties *string `json:"properties"`
}

// ToEntity Converts BluecatEntity to Entity
func (er *BluecatEntity) ToEntity() Entity {
	// Handle nil pointer dereference if name or properties is null
	// Type is guaranteed to be non-nil unless entity does not exist, in which case it is handled earlier
	var name string
	var properties map[string]string

	if er.Name != nil {
		name = *er.Name
	}

	if er.Properties != nil {
		properties = common.ConvertToMap(*er.Properties, "|")
	} else {
		properties = make(map[string]string)
	}

	// Convert BluecatEntity to Entity
	entity := Entity{
		ID:         er.ID,
		Name:       name,
		Type:       *er.Type,
		Properties: properties,
	}

	return entity
}

// ConvertToEntities Converts a slice of BluecatEntities to a slice of Entities
func ConvertToEntities(BluecatEntities []BluecatEntity) []Entity {
	entities := make([]Entity, len(BluecatEntities))
	for i, bluecatEntity := range BluecatEntities {
		entities[i] = bluecatEntity.ToEntity()
	}

	return entities
}

// IsEmpty Checks if an BluecatEntity is empty
func (er *BluecatEntity) IsEmpty() bool {
	return er.ID == 0 && er.Name == nil && er.Type == nil && er.Properties == nil
}
