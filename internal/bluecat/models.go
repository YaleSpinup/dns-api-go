package bluecat

import "dns-api-go/internal/models"

// V2Entity represents a Bluecat Address Manager V2 API entity response.
// In V2, properties are first-class JSON fields rather than pipe-delimited strings.
type V2Entity struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties,omitempty"`

	// Common V2 fields that may appear as top-level JSON
	Configuration *V2Link `json:"configuration,omitempty"`
	Parent        *V2Link `json:"parent,omitempty"`

	// V2 includes userDefinedFields as a separate map
	UserDefinedFields map[string]string `json:"userDefinedFields,omitempty"`
}

// V2Link represents a linked entity reference in V2 responses.
type V2Link struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// V2Collection represents a paginated collection response from the V2 API.
type V2Collection struct {
	Count int        `json:"count"`
	Data  []V2Entity `json:"data"`
}

// V2SessionResponse represents the response from POST /api/v2/sessions.
type V2SessionResponse struct {
	APIToken    string `json:"apiToken"`
	APIUser     string `json:"apiUser,omitempty"`
	LogoutURL   string `json:"logoutUrl,omitempty"`
	BasicAuthCr string `json:"basicAuthenticationCredentials,omitempty"`
}

// V2ErrorResponse represents an error response from the V2 API.
type V2ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToEntity converts a V2Entity to the internal models.Entity type.
// This preserves compatibility with existing handlers and services.
func (e *V2Entity) ToEntity() models.Entity {
	properties := make(map[string]string)

	// Copy V2 properties
	for k, v := range e.Properties {
		properties[k] = v
	}

	// Copy user-defined fields into properties (matching V1 behavior)
	for k, v := range e.UserDefinedFields {
		properties[k] = v
	}

	return models.Entity{
		ID:         e.ID,
		Name:       e.Name,
		Type:       e.Type,
		Properties: properties,
	}
}

// ConvertV2ToEntities converts a slice of V2Entity to a slice of models.Entity.
func ConvertV2ToEntities(v2Entities []V2Entity) []models.Entity {
	entities := make([]models.Entity, len(v2Entities))
	for i, v2e := range v2Entities {
		entities[i] = v2e.ToEntity()
	}
	return entities
}

// IsEmpty checks if a V2Entity represents an empty/not-found response.
func (e *V2Entity) IsEmpty() bool {
	return e.ID == 0 && e.Name == "" && e.Type == ""
}

