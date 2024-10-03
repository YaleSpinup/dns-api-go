package models

import (
	"dns-api-go/internal/common"
	"testing"
)

func TestToEntity(t *testing.T) {
	tests := []struct {
		name           string
		BluecatEntity  BluecatEntity
		expectedEntity Entity
	}{
		{
			name: "All fields present",
			BluecatEntity: BluecatEntity{
				ID:         1,
				Name:       common.StringPtr("Test Entity"),
				Type:       common.StringPtr("HostRecord"),
				Properties: common.StringPtr("key1=value1|key2=value2"),
			},
			expectedEntity: Entity{
				ID:   1,
				Name: "Test Entity",
				Type: "HostRecord",
				Properties: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "Name and Properties are nil",
			BluecatEntity: BluecatEntity{
				ID:         1,
				Name:       nil,
				Type:       common.StringPtr("HostRecord"),
				Properties: nil,
			},
			expectedEntity: Entity{
				ID:         1,
				Name:       "",
				Type:       "HostRecord",
				Properties: map[string]string{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entity := tc.BluecatEntity.ToEntity()
			common.CheckResponse(t, tc.name, tc.expectedEntity, entity)
		})
	}
}

func TestConvertToEntities(t *testing.T) {
	tests := []struct {
		name             string
		BluecatEntities  []BluecatEntity
		expectedEntities []Entity
	}{
		{
			name: "Multiple entities",
			BluecatEntities: []BluecatEntity{
				{
					ID:         1,
					Name:       common.StringPtr("Entity1"),
					Type:       common.StringPtr("Type1"),
					Properties: common.StringPtr("key1=value1|key2=value2"),
				},
				{
					ID:         2,
					Name:       common.StringPtr("Entity2"),
					Type:       common.StringPtr("Type2"),
					Properties: common.StringPtr("key3=value3|key4=value4"),
				},
			},
			expectedEntities: []Entity{
				{
					ID:   1,
					Name: "Entity1",
					Type: "Type1",
					Properties: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
				{
					ID:   2,
					Name: "Entity2",
					Type: "Type2",
					Properties: map[string]string{
						"key3": "value3",
						"key4": "value4",
					},
				},
			},
		},
		{
			name:             "Empty entity response slice",
			BluecatEntities:  []BluecatEntity{},
			expectedEntities: []Entity{},
		},
		{
			name: "Single entity with nil fields",
			BluecatEntities: []BluecatEntity{
				{
					ID:         3,
					Name:       nil,
					Type:       common.StringPtr("Type3"),
					Properties: nil,
				},
			},
			expectedEntities: []Entity{
				{
					ID:         3,
					Name:       "",
					Type:       "Type3",
					Properties: map[string]string{},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entities := ConvertToEntities(tc.BluecatEntities)
			common.CheckResponse(t, tc.name, tc.expectedEntities, entities)
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name         string
		er           BluecatEntity
		expectedResp bool
	}{
		{
			name:         "Completely empty BluecatEntity",
			er:           BluecatEntity{},
			expectedResp: true,
		},
		{
			name: "BluecatEntity with all fields explicitly set to zero/nil",
			er: BluecatEntity{
				ID:         0,
				Name:       nil,
				Type:       nil,
				Properties: nil,
			},
			expectedResp: true,
		},
		{
			name:         "BluecatEntity with only ID set",
			er:           BluecatEntity{ID: 1},
			expectedResp: false,
		},
		{
			name:         "BluecatEntity with only Name set",
			er:           BluecatEntity{Name: common.StringPtr("Test")},
			expectedResp: false,
		},
		{
			name:         "BluecatEntity with only Type set",
			er:           BluecatEntity{Type: common.StringPtr("TestType")},
			expectedResp: false,
		},
		{
			name:         "BluecatEntity with only Properties set",
			er:           BluecatEntity{Properties: common.StringPtr("Test properties")},
			expectedResp: false,
		},
		{
			name: "BluecatEntity with all fields set",
			er: BluecatEntity{
				ID:         1,
				Name:       common.StringPtr("Test"),
				Type:       common.StringPtr("TestType"),
				Properties: common.StringPtr("Test properties"),
			},
			expectedResp: false,
		},
		{
			name: "BluecatEntity with zero ID and other fields set",
			er: BluecatEntity{
				ID:         0,
				Name:       common.StringPtr("Test"),
				Type:       common.StringPtr("TestType"),
				Properties: common.StringPtr("Test properties"),
			},
			expectedResp: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if res := tc.er.IsEmpty(); res != tc.expectedResp {
				t.Errorf("isEmpty() = %v, expectedResp %v", res, tc.expectedResp)
			}
		})
	}
}
