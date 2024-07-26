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
				"type": "HOSTRECORD",
				"properties": "TestProperties"
			}`),
			mockMakeRequestError: nil,
			expectedResponse: &models.Entity{
				ID:         1,
				Name:       "Test Entity",
				Type:       "HOSTRECORD",
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
				MakeRequestFunc: func(route, queryParam string) ([]byte, error) {
					return tc.mockMakeRequestResponse, tc.mockMakeRequestError
				},
			}

			entityService := NewGenericEntityService(mockServer)
			entity, err := entityService.GetEntityByID(tc.entityId, tc.includeHA)

			// Check for specific ErrEntityNotFound error
			if tc.expectedError != nil && tc.expectedError.Error() == "Simulating unmarshal error" {
				if err == nil {
					t.Errorf("%s: expected an unmarshal error, got nil", tc.name)
				}
				// For other errors, just check that an error was returned
			} else if tc.expectedError != nil {
				if !errors.Is(err, tc.expectedError) {
					t.Errorf("%s: expected error %v, got %v", tc.name, tc.expectedError, err)
				}
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
				Type:       common.StringPtr("HOSTRECORD"),
				Properties: common.StringPtr("Test Properties"),
			},
			expectedEntity: &models.Entity{
				ID:         1,
				Name:       "Test Entity",
				Type:       "HOSTRECORD",
				Properties: "Test Properties",
			},
		},
		{
			name: "Name and Properties are nil",
			entityResponse: EntityResponse{
				ID:         1,
				Name:       nil,
				Type:       common.StringPtr("HOSTRECORD"),
				Properties: nil,
			},
			expectedEntity: &models.Entity{
				ID:         1,
				Name:       "",
				Type:       "HOSTRECORD",
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
				"type": "HOSTRECORD",
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
				"type": "HOSTRECORD",
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
				"type": "HOSTRECORD",
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
				MakeRequestFunc: func(route, queryParam string) ([]byte, error) {
					if strings.Contains(route, "getEntityById") {
						return tc.mockMakeReqGetEntByIDResp, tc.mockMakeReqGetEntByIDError
					} else if strings.Contains(route, "delete") {
						return nil, tc.mockMakeReqDelEntByIDError
					} else {
						return nil, errors.New("unexpected route")
					}
				},
			}

			entityService := NewGenericEntityService(mockServer)
			err := entityService.DeleteEntityByID(tc.entityId)
			if tc.expectedError != nil && !errors.Is(err, tc.expectedError) {
				t.Errorf("%s: expected error %v, got %v", tc.name, tc.expectedError, err)
			} else if tc.expectedError == nil && err != nil {
				t.Errorf("%s: expected no error, got %v", tc.name, err)
			}
		})
	}
}
