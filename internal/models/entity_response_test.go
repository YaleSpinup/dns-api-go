package models

import (
	"dns-api-go/internal/common"
	"reflect"
	"testing"
)

func TestToEntity(t *testing.T) {
	tests := []struct {
		name           string
		entityResponse EntityResponse
		expectedEntity Entity
	}{
		{
			name: "All fields present",
			entityResponse: EntityResponse{
				ID:         1,
				Name:       common.StringPtr("Test Entity"),
				Type:       common.StringPtr("HostRecord"),
				Properties: common.StringPtr("Test Properties"),
			},
			expectedEntity: Entity{
				ID:         1,
				Name:       "Test Entity",
				Type:       "HostRecord",
				Properties: "Test Properties",
			},
		},
		{
			name: "Name and Properties are nil",
			entityResponse: EntityResponse{
				ID:         1,
				Name:       nil,
				Type:       common.StringPtr("HostRecord"),
				Properties: nil,
			},
			expectedEntity: Entity{
				ID:         1,
				Name:       "",
				Type:       "HostRecord",
				Properties: "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entity := tc.entityResponse.ToEntity()
			if !reflect.DeepEqual(entity, tc.expectedEntity) {
				t.Errorf("%s: expected response %+v, got %+v", tc.name, tc.expectedEntity, entity)
			}
		})
	}
}

func TestConvertToEntities(t *testing.T) {
	tests := []struct {
		name             string
		entityResponses  []EntityResponse
		expectedEntities []Entity
	}{
		{
			name: "Multiple entities",
			entityResponses: []EntityResponse{
				{
					ID:         1,
					Name:       common.StringPtr("Entity1"),
					Type:       common.StringPtr("Type1"),
					Properties: common.StringPtr("Properties1"),
				},
				{
					ID:         2,
					Name:       common.StringPtr("Entity2"),
					Type:       common.StringPtr("Type2"),
					Properties: common.StringPtr("Properties2"),
				},
			},
			expectedEntities: []Entity{
				{
					ID:         1,
					Name:       "Entity1",
					Type:       "Type1",
					Properties: "Properties1",
				},
				{
					ID:         2,
					Name:       "Entity2",
					Type:       "Type2",
					Properties: "Properties2",
				},
			},
		},
		{
			name:             "Empty entity response slice",
			entityResponses:  []EntityResponse{},
			expectedEntities: []Entity{},
		},
		{
			name: "Single entity with nil fields",
			entityResponses: []EntityResponse{
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
					Properties: "",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entities := ConvertToEntities(tc.entityResponses)
			if !reflect.DeepEqual(entities, tc.expectedEntities) {
				t.Errorf("ConvertToEntities() = %+v, expected %+v", entities, tc.expectedEntities)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name         string
		er           EntityResponse
		expectedResp bool
	}{
		{
			name:         "Completely empty EntityResponse",
			er:           EntityResponse{},
			expectedResp: true,
		},
		{
			name: "EntityResponse with all fields explicitly set to zero/nil",
			er: EntityResponse{
				ID:         0,
				Name:       nil,
				Type:       nil,
				Properties: nil,
			},
			expectedResp: true,
		},
		{
			name:         "EntityResponse with only ID set",
			er:           EntityResponse{ID: 1},
			expectedResp: false,
		},
		{
			name:         "EntityResponse with only Name set",
			er:           EntityResponse{Name: common.StringPtr("Test")},
			expectedResp: false,
		},
		{
			name:         "EntityResponse with only Type set",
			er:           EntityResponse{Type: common.StringPtr("TestType")},
			expectedResp: false,
		},
		{
			name:         "EntityResponse with only Properties set",
			er:           EntityResponse{Properties: common.StringPtr("Test properties")},
			expectedResp: false,
		},
		{
			name: "EntityResponse with all fields set",
			er: EntityResponse{
				ID:         1,
				Name:       common.StringPtr("Test"),
				Type:       common.StringPtr("TestType"),
				Properties: common.StringPtr("Test properties"),
			},
			expectedResp: false,
		},
		{
			name: "EntityResponse with zero ID and other fields set",
			er: EntityResponse{
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
