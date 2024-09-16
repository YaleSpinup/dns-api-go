package services

import (
	"dns-api-go/internal/common"
	"dns-api-go/internal/mocks"
	"dns-api-go/internal/models"
	"github.com/pkg/errors"
	"io"
	"os"
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
				"properties": "key1=value1|key2=value2"
			}`),
			mockMakeRequestError: nil,
			expectedResponse: &models.Entity{
				ID:   1,
				Name: "Test Entity",
				Type: "HostRecord",
				Properties: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
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
				MakeRequestFunc: func(method, route, queryParam string, body io.Reader) ([]byte, error) {
					return tc.mockMakeRequestResponse, tc.mockMakeRequestError
				},
			}

			entityService := NewBaseService(mockServer)
			entity, err := entityService.GetEntity(tc.entityId, tc.includeHA)

			common.CheckError(t, tc.name, tc.expectedError, err)
			common.CheckResponse(t, tc.name, tc.expectedResponse, entity)
		})
	}
}

func TestDeleteEntity(t *testing.T) {
	tests := []struct {
		name                   string
		entityId               int
		mockMakeReqGetEntResp  []byte
		mockMakeReqGetEntError error
		mockMakeReqDelEntError error
		expectedError          error
	}{
		{
			name:     "Successful deletion with expected types",
			entityId: 1,
			mockMakeReqGetEntResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "HostRecord",
				"properties": "key1=value1|key2=value2"
			}`),
			mockMakeReqGetEntError: nil,
			mockMakeReqDelEntError: nil,
			expectedError:          nil,
		},
		{
			name:     "Successful deletion with empty expected types",
			entityId: 1,
			mockMakeReqGetEntResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "HostRecord",
				"properties": "key1=value1|key2=value2"
			}`),
			mockMakeReqGetEntError: nil,
			mockMakeReqDelEntError: nil,
			expectedError:          nil,
		},
		{
			name:                   "GetEntity error",
			entityId:               999,
			mockMakeReqGetEntResp:  nil,
			mockMakeReqGetEntError: errors.New("Simulating GetEntity error"),
			mockMakeReqDelEntError: nil,
			expectedError:          errors.New("Simulating GetEntity error"),
		},
		{
			name:     "Entity deletion not allowed",
			entityId: 1,
			mockMakeReqGetEntResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "INVALIDTYPE",
				"properties": "key1=value1|key2=value2"
			}`),
			mockMakeReqGetEntError: nil,
			mockMakeReqDelEntError: nil,
			expectedError:          &ErrDeleteNotAllowed{Type: "INVALIDTYPE"},
		},
		{
			name:     "MakeRequest error",
			entityId: 1,
			mockMakeReqGetEntResp: []byte(`{
				"id": 1,
				"name": "Test Entity",
				"type": "HostRecord",
				"properties": "key1=value1|key2=value2"
			}`),
			mockMakeReqGetEntError: nil,
			mockMakeReqDelEntError: errors.New("Simulating MakeRequest error"),
			expectedError:          errors.New("Simulating MakeRequest error"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := &mocks.MockServer{
				MakeRequestFunc: func(method, route, queryParam string, body io.Reader) ([]byte, error) {
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
			err := entityService.DeleteEntity(tc.entityId)

			common.CheckError(t, tc.name, tc.expectedError, err)
		})
	}
}
