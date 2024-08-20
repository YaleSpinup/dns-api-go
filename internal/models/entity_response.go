package models

type EntityResponse struct {
	ID         int     `json:"id"`
	Name       *string `json:"name"`
	Type       *string `json:"type"`
	Properties *string `json:"properties"`
}

// ToEntity Converts EntityResponse to Entity
func (er *EntityResponse) ToEntity() Entity {
	// Handle nil pointer dereference if name or properties is null
	// Type is guaranteed to be non-nil unless entity does not exist, in which case it is handled earlier
	var name, properties string
	if er.Name != nil {
		name = *er.Name
	}
	if er.Properties != nil {
		properties = *er.Properties
	}

	// Convert EntityResponse to Entity
	entity := Entity{
		ID:         er.ID,
		Name:       name,
		Type:       *er.Type,
		Properties: properties,
	}

	return entity
}

// ConvertToEntities Converts a slice of EntityResponses to a slice of Entities
func ConvertToEntities(entityResponses []EntityResponse) []Entity {
	entities := make([]Entity, len(entityResponses))
	for i, entityResp := range entityResponses {
		entities[i] = entityResp.ToEntity()
	}

	return entities
}

// IsEmpty Checks if an EntityResponse is empty
func (er *EntityResponse) IsEmpty() bool {
	return er.ID == 0 && er.Name == nil && er.Type == nil && er.Properties == nil
}
