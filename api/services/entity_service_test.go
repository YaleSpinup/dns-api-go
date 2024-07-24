package services

import (
	"dns-api-go/api/mocks"
	"dns-api-go/common"
	"github.com/pkg/errors"
	"os"
	"reflect"
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
		mockMakeRequestResponse string
		mockMakeRequestError    error
		expectedResponse        *Entity
		expectedError           error
	}{
		{
			name:      "Successful retrieval",
			entityId:  1,
			includeHA: true,
			mockMakeRequestResponse: `{
				"id": 1,
				"name": "Test Entity",
				"type": "HOSTRECORD",
				"properties": "TestProperties"
			}`,
			mockMakeRequestError: nil,
			expectedResponse: &Entity{
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
			mockMakeRequestResponse: `{
				"id": 0,
				"name": null,
				"type": null,
				"properties": null
			}`,
			mockMakeRequestError: nil,
			expectedResponse:     nil,
			expectedError:        &ErrEntityNotFound{},
		},
		{
			name:                    "JSON unmarshal error",
			entityId:                1,
			includeHA:               true,
			mockMakeRequestResponse: "{invalidJson}",
			mockMakeRequestError:    nil,
			expectedResponse:        nil,
			expectedError:           errors.New("simulating unmarshal error"),
		},
		{
			name:                    "MakeRequest Error",
			entityId:                -1,
			includeHA:               true,
			mockMakeRequestResponse: "",
			mockMakeRequestError:    errors.New("simulating server response error"),
			expectedResponse:        nil,
			expectedError:           errors.New("simulating server response error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := &mocks.MockServer{
				MakeRequestFunc: func(route, queryParam string) ([]byte, error) {
					return []byte(tc.mockMakeRequestResponse), tc.mockMakeRequestError
				},
			}

			entityService := NewGenericEntityService(mockServer)
			entity, err := entityService.GetEntityByID(tc.entityId, tc.includeHA)

			// Check for specific ErrEntityNotFound error
			if _, ok := tc.expectedError.(*ErrEntityNotFound); ok {
				if !errors.Is(err, tc.expectedError) {
					t.Errorf("%s: expected ErrEntityNotFound, got %v", tc.name, err)
				}
				// For other errors, just check that an error was returned
			} else if tc.expectedError != nil && err == nil {
				t.Errorf("%s: expected an error, got nil", tc.name)
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
		expectedEntity *Entity
	}{
		{
			name: "All fields present",
			entityResponse: EntityResponse{
				ID:         1,
				Name:       common.StringPtr("Test Entity"),
				Type:       common.StringPtr("HOSTRECORD"),
				Properties: common.StringPtr("Test Properties"),
			},
			expectedEntity: &Entity{
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
			expectedEntity: &Entity{
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
