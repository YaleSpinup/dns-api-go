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

func TestGetEntity(t *testing.T) {
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
			entity, err := entityService.GetEntity(tc.entityId, tc.includeHA)

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

func TestDeleteEntity(t *testing.T) {
	tests := []struct {
		name                   string
		entityId               int
		expectedTypes          []string
		mockMakeReqGetEntResp  []byte
		mockMakeReqGetEntError error
		mockMakeReqDelEntError error
		expectedError          error
	}{
		{
			name:          "Successful deletion with expected types",
			entityId:      1,
			expectedTypes: []string{"HostRecord", "AliasRecord"},
			mockMakeReqGetEntResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "HostRecord",
				"properties": "TestProperties"
			}`),
			mockMakeReqGetEntError: nil,
			mockMakeReqDelEntError: nil,
			expectedError:          nil,
		},
		{
			name:          "Successful deletion with empty expected types",
			entityId:      1,
			expectedTypes: []string{},
			mockMakeReqGetEntResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "HostRecord",
				"properties": "TestProperties"
			}`),
			mockMakeReqGetEntError: nil,
			mockMakeReqDelEntError: nil,
			expectedError:          nil,
		},
		{
			name:                   "GetEntity error",
			entityId:               999,
			expectedTypes:          []string{"HostRecord"},
			mockMakeReqGetEntResp:  nil,
			mockMakeReqGetEntError: errors.New("Simulating GetEntity error"),
			mockMakeReqDelEntError: nil,
			expectedError:          errors.New("Simulating GetEntity error"),
		},
		{
			name:          "Entity type mismatch",
			entityId:      1,
			expectedTypes: []string{"HostRecord"},
			mockMakeReqGetEntResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "INVALIDTYPE",
				"properties": "TestProperties"
			}`),
			mockMakeReqGetEntError: nil,
			mockMakeReqDelEntError: nil,
			expectedError:          &ErrEntityTypeMismatch{ExpectedTypes: []string{"HostRecord"}, ActualType: "INVALIDTYPE"},
		},
		{
			name:          "Entity deletion not allowed",
			entityId:      1,
			expectedTypes: []string{},
			mockMakeReqGetEntResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "INVALIDTYPE",
				"properties": "TestProperties"
			}`),
			mockMakeReqGetEntError: nil,
			mockMakeReqDelEntError: nil,
			expectedError:          &ErrDeleteNotAllowed{Type: "INVALIDTYPE"},
		},
		{
			name:          "MakeRequest error",
			entityId:      1,
			expectedTypes: []string{"HostRecord"},
			mockMakeReqGetEntResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "HostRecord",
				"properties": "TestProperties"
			}`),
			mockMakeReqGetEntError: nil,
			mockMakeReqDelEntError: errors.New("Simulating MakeRequest error"),
			expectedError:          errors.New("Simulating MakeRequest error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := &mocks.MockServer{
				MakeRequestFunc: func(method, route, queryParam string) ([]byte, error) {
					if strings.Contains(route, "getEntityById") {
						return tc.mockMakeReqGetEntResp, tc.mockMakeReqGetEntError
					} else if strings.Contains(route, "delete") {
						return nil, tc.mockMakeReqDelEntError
					} else {
						return nil, errors.New("unexpected route")
					}
				},
			}

			entityService := NewBaseService(mockServer)
			err := entityService.DeleteEntity(tc.entityId, tc.expectedTypes)
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
