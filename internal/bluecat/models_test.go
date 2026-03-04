package bluecat

import (
	"encoding/json"
	"testing"
)

func TestV2Entity_ToEntity(t *testing.T) {
	v2 := V2Entity{
		ID:   42,
		Name: "test-zone",
		Type: "Zone",
		Properties: map[string]string{
			"absoluteName": "test.example.com",
		},
		UserDefinedFields: map[string]string{
			"customField": "customValue",
		},
	}

	entity := v2.ToEntity()

	if entity.ID != 42 {
		t.Errorf("expected ID 42, got %d", entity.ID)
	}
	if entity.Name != "test-zone" {
		t.Errorf("expected name 'test-zone', got '%s'", entity.Name)
	}
	if entity.Type != "Zone" {
		t.Errorf("expected type 'Zone', got '%s'", entity.Type)
	}
	if entity.Properties["absoluteName"] != "test.example.com" {
		t.Errorf("expected absoluteName property, got %v", entity.Properties)
	}
	if entity.Properties["customField"] != "customValue" {
		t.Errorf("expected customField in properties, got %v", entity.Properties)
	}
}

func TestV2Entity_IsEmpty(t *testing.T) {
	empty := V2Entity{}
	if !empty.IsEmpty() {
		t.Error("expected empty entity to be empty")
	}

	nonEmpty := V2Entity{ID: 1, Name: "test", Type: "Zone"}
	if nonEmpty.IsEmpty() {
		t.Error("expected non-empty entity to not be empty")
	}
}

func TestConvertV2ToEntities(t *testing.T) {
	v2Entities := []V2Entity{
		{ID: 1, Name: "zone1", Type: "Zone"},
		{ID: 2, Name: "zone2", Type: "Zone"},
	}

	entities := ConvertV2ToEntities(v2Entities)

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
	if entities[0].ID != 1 || entities[1].ID != 2 {
		t.Errorf("unexpected entity IDs: %d, %d", entities[0].ID, entities[1].ID)
	}
}

func TestV2Collection_Unmarshal(t *testing.T) {
	jsonData := `{"count":2,"data":[{"id":1,"name":"zone1","type":"Zone"},{"id":2,"name":"zone2","type":"Zone"}]}`

	var collection V2Collection
	if err := json.Unmarshal([]byte(jsonData), &collection); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if collection.Count != 2 {
		t.Errorf("expected count 2, got %d", collection.Count)
	}
	if len(collection.Data) != 2 {
		t.Errorf("expected 2 data items, got %d", len(collection.Data))
	}
}

func TestV2SessionResponse_Unmarshal(t *testing.T) {
	jsonData := `{"apiToken":"abc123","apiUser":"admin"}`

	var resp V2SessionResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.APIToken != "abc123" {
		t.Errorf("expected apiToken 'abc123', got '%s'", resp.APIToken)
	}
	if resp.APIUser != "admin" {
		t.Errorf("expected apiUser 'admin', got '%s'", resp.APIUser)
	}
}

