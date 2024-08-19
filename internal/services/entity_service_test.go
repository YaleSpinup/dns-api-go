package services

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/mocks"
	"dns-api-go/internal/models"
	"github.com/pkg/errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Setup phase: Initialize the logger
	common.SetupLogger()

	// Run the tests
	code := m.Run()

	// Exit with the code from m.Run()
	os.Exit(code)
}

func TestGetEntityByID(t *testing.T) {
	tests := []struct {
		name                    string
		entityId                int
		includeHA               bool
		mockMakeRequestResponse []byte
		mockMakeRequestError    error
		expectedResponse        *models.Entity
		expectedError           error
	}{
		{
			name:      "Successful retrieval",
			entityId:  1,
			includeHA: true,
			mockMakeRequestResponse: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "HostRecord",
				"properties": "TestProperties"
			}`),
			mockMakeRequestError: nil,
			expectedResponse: &models.Entity{
				ID:         1,
				Name:       "Test Entity",
				Type:       "HostRecord",
				Properties: "TestProperties",
			},
			expectedError: nil,
		},
		{
			name:      "Entity not found",
			entityId:  999,
			includeHA: true,
			mockMakeRequestResponse: []byte(`{
				"id": 0,
				"name": null,
				"type": null,
				"properties": null
			}`),
			mockMakeRequestError: nil,
			expectedResponse:     nil,
			expectedError:        &ErrEntityNotFound{},
		},
		{
			name:                    "JSON unmarshal error",
			entityId:                1,
			includeHA:               true,
			mockMakeRequestResponse: []byte("{invalidJson}"),
			mockMakeRequestError:    nil,
			expectedResponse:        nil,
			expectedError:           errors.New("Simulating unmarshal error"),
		},
		{
			name:                    "MakeRequest Error",
			entityId:                -1,
			includeHA:               true,
			mockMakeRequestResponse: nil,
			mockMakeRequestError:    errors.New("Simulating MakeRequest error"),
			expectedResponse:        nil,
			expectedError:           errors.New("Simulating MakeRequest error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := &mocks.MockServer{
				MakeRequestFunc: func(method, route, queryParam string) ([]byte, error) {
					return tc.mockMakeRequestResponse, tc.mockMakeRequestError
				},
			}

			entityService := NewBaseService(mockServer)
			entity, err := entityService.GetEntityByID(tc.entityId, tc.includeHA)

			// If json unmarshalling error, check that any error is returned
			if tc.expectedError != nil && tc.expectedError.Error() == "Simulating unmarshal error" {
				if err == nil {
					t.Errorf("%s: expected an unmarshal error, got nil", tc.name)
				}
				// For other errors, check if returned error matches expected error
			} else if tc.expectedError != nil && !common.CompareErrors(tc.expectedError, err) {
				t.Errorf("%s: expected error %v, got %v", tc.name, tc.expectedError, err)
				// No error expected
			} else if tc.expectedError == nil && err != nil {
				t.Errorf("%s: expected no error, got %v", tc.name, err)
			}

			// Check the response
			if !reflect.DeepEqual(entity, tc.expectedResponse) {
				t.Errorf("%s: expected response %+v, got %+v", tc.name, tc.expectedResponse, entity)
			}
		})
	}
}

func TestToEntity(t *testing.T) {
	tests := []struct {
		name           string
		entityResponse EntityResponse
		expectedEntity *models.Entity
	}{
		{
			name: "All fields present",
			entityResponse: EntityResponse{
				ID:         1,
				Name:       common.StringPtr("Test Entity"),
				Type:       common.StringPtr("HostRecord"),
				Properties: common.StringPtr("Test Properties"),
			},
			expectedEntity: &models.Entity{
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
			expectedEntity: &models.Entity{
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
			if res := tc.er.isEmpty(); res != tc.expectedResp {
				t.Errorf("isEmpty() = %v, expectedResp %v", res, tc.expectedResp)
			}
		})
	}
}

func TestDeleteEntityByID(t *testing.T) {
	tests := []struct {
		name                       string
		entityId                   int
		mockMakeReqGetEntByIDResp  []byte
		mockMakeReqGetEntByIDError error
		mockMakeReqDelEntByIDError error
		expectedError              error
	}{
		{
			name:     "Successful deletion",
			entityId: 1,
			mockMakeReqGetEntByIDResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "HostRecord",
				"properties": "TestProperties"
			}`),
			mockMakeReqGetEntByIDError: nil,
			mockMakeReqDelEntByIDError: nil,
			expectedError:              nil,
		},
		{
			name:                       "GetEntityByID error",
			entityId:                   999,
			mockMakeReqGetEntByIDResp:  nil,
			mockMakeReqGetEntByIDError: errors.New("Simulating GetEntityByID error"),
			mockMakeReqDelEntByIDError: nil,
			expectedError:              errors.New("Simulating GetEntityByID error"),
		},
		{
			name:     "Entity deletion not allowed",
			entityId: 1,
			mockMakeReqGetEntByIDResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "INVALIDTYPE",
				"properties": "TestProperties"
			}`),
			mockMakeReqGetEntByIDError: nil,
			mockMakeReqDelEntByIDError: nil,
			expectedError:              &ErrDeleteNotAllowed{Type: "INVALIDTYPE"},
		},
		{
			name:     "MakeRequest error",
			entityId: 1,
			mockMakeReqGetEntByIDResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "HostRecord",
				"properties": "TestProperties"
			}`),
			mockMakeReqGetEntByIDError: nil,
			mockMakeReqDelEntByIDError: errors.New("Simulating MakeRequest error"),
			expectedError:              errors.New("Simulating MakeRequest error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := &mocks.MockServer{
				MakeRequestFunc: func(method, route, queryParam string) ([]byte, error) {
					if strings.Contains(route, "getEntityById") {
						return tc.mockMakeReqGetEntByIDResp, tc.mockMakeReqGetEntByIDError
					} else if strings.Contains(route, "delete") {
						return nil, tc.mockMakeReqDelEntByIDError
					} else {
						return nil, errors.New("unexpected route")
					}
				},
			}

			entityService := NewBaseService(mockServer)
			err := entityService.DeleteEntityByID(tc.entityId)
			if tc.expectedError != nil && !common.CompareErrors(tc.expectedError, err) {
				t.Errorf("%s: expected error %v, got %v", tc.name, tc.expectedError, err)
			} else if tc.expectedError == nil && err != nil {
				t.Errorf("%s: expected no error, got %v", tc.name, err)
			}
		})
	}
}

func TestGetEntities(t *testing.T) {
	tests := []struct {
		name                    string
		start                   int
		count                   int
		parentId                int
		entityType              string
		includeHA               bool
		mockMakeRequestResponse []byte
		mockMakeRequestError    error
		expectedResponse        *[]models.Entity
		expectedError           error
	}{
		{
			name:       "Successful retrieval",
			start:      0,
			count:      2,
			parentId:   1,
			entityType: "HostRecord",
			includeHA:  true,
			mockMakeRequestResponse: []byte(`[
                {
                    "id": 1,
                    "name": "Entity1",
                    "type": "HostRecord",
                    "properties": "Properties1"
                },
                {
                    "id": 2,
                    "name": "Entity2",
                    "type": "HostRecord",
                    "properties": "Properties2"
                }
            ]`),
			mockMakeRequestError: nil,
			expectedResponse: &[]models.Entity{
				{
					ID:         1,
					Name:       "Entity1",
					Type:       "HostRecord",
					Properties: "Properties1",
				},
				{
					ID:         2,
					Name:       "Entity2",
					Type:       "HostRecord",
					Properties: "Properties2",
				},
			},
			expectedError: nil,
		},
		{
			name:                    "Empty result",
			start:                   0,
			count:                   10,
			parentId:                999,
			entityType:              "HostRecord",
			includeHA:               false,
			mockMakeRequestResponse: []byte(`[]`),
			mockMakeRequestError:    nil,
			expectedResponse:        &[]models.Entity{},
			expectedError:           nil,
		},
		{
			name:                    "JSON unmarshal error",
			start:                   0,
			count:                   10,
			parentId:                1,
			entityType:              "HostRecord",
			includeHA:               true,
			mockMakeRequestResponse: []byte("{invalidJson}"),
			mockMakeRequestError:    nil,
			expectedResponse:        nil,
			expectedError:           errors.New("Simulating unmarshal error"),
		},
		{
			name:                    "MakeRequest Error",
			start:                   -1,
			count:                   10,
			parentId:                1,
			entityType:              "InvalidType",
			includeHA:               true,
			mockMakeRequestResponse: nil,
			mockMakeRequestError:    errors.New("Simulating MakeRequest error"),
			expectedResponse:        nil,
			expectedError:           errors.New("Simulating MakeRequest error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := &mocks.MockServer{
				MakeRequestFunc: func(method, route, queryParam string) ([]byte, error) {
					return tc.mockMakeRequestResponse, tc.mockMakeRequestError
				},
			}

			entityService := NewBaseService(mockServer)
			entities, err := entityService.GetEntities(tc.start, tc.count, tc.parentId, tc.entityType, tc.includeHA)

			// If json unmarshalling error, check that any error is returned
			if tc.expectedError != nil && tc.expectedError.Error() == "Simulating unmarshal error" {
				if err == nil {
					t.Errorf("%s: expected an unmarshal error, got nil", tc.name)
				}
				// For other errors, check if returned error matches expected error
			} else if tc.expectedError != nil && !common.CompareErrors(tc.expectedError, err) {
				t.Errorf("%s: expected error %v, got %v", tc.name, tc.expectedError, err)
				// No error expected
			} else if tc.expectedError == nil && err != nil {
				t.Errorf("%s: expected no error, got %v", tc.name, err)
			}

			// Check the response
			if !reflect.DeepEqual(entities, tc.expectedResponse) {
				t.Errorf("%s: expected response %+v, got %+v", tc.name, tc.expectedResponse, entities)
			}
		})
	}
}
