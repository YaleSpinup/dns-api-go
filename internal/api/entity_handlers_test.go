package api

import (
	"dns-api-go/internal/models"
	"dns-api-go/internal/services"
	"errors"
	"net/http"
	"testing"
)

func TestGetEntityIDHandler(t *testing.T) {
	tests := []struct {
		name           string
		id             string
		includeHA      string
		mockEntity     *models.Entity
		mockError      error
		expectedBody   string
		expectedStatus int
	}{
		{
			name:      "Successful retrieval",
			id:        "1",
			includeHA: "true",
			mockEntity: &models.Entity{
				ID:         1,
				Name:       "Test Entity",
				Type:       "TestType",
				Properties: "TestProperties",
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"ID":1,"Name":"Test Entity","Type":"TestType","Properties":"TestProperties"}`,
		},
		{
			name:      "Missing includeHA",
			id:        "1",
			includeHA: "",
			mockEntity: &models.Entity{
				ID:         1,
				Name:       "Test Entity",
				Type:       "TestType",
				Properties: "TestProperties",
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"ID":1,"Name":"Test Entity","Type":"TestType","Properties":"TestProperties"}`,
		},
		{
			name:           "Invalid ID format",
			id:             "invalid",
			includeHA:      "true",
			mockEntity:     nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid ID format\n",
		},
		{
			name:           "Entity not found",
			id:             "999",
			includeHA:      "true",
			mockEntity:     nil,
			mockError:      &services.ErrEntityNotFound{},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Entity not found\n",
		},
		{
			name:           "GetEntityByID server error",
			id:             "2",
			includeHA:      "true",
			mockEntity:     nil,
			mockError:      errors.New("internal error"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal error\n",
		},
		{
			name:           "Marshaling error",
			id:             "1",
			includeHA:      "true",
			mockEntity:     &models.Entity{},
			mockError:      nil,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal server error\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// TODO: Implement the test
		})
	}
}
