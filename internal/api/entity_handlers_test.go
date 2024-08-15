package api

import (
	"dns-api-go/internal/models"
	"dns-api-go/internal/services"
	"errors"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestParseEntityParams(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		queryParams    url.Values
		expectedParams *EntityParams
		expectedError  string
	}{
		{
			name: "Valid ID and includeHA",
			url:  "/id/1",
			queryParams: url.Values{
				"includeHA": []string{"false"},
			},
			expectedParams: &EntityParams{
				ID:        1,
				IncludeHA: false,
			},
			expectedError: "",
		},
		{
			name:        "Valid ID without includeHA",
			url:         "/id/1",
			queryParams: url.Values{},
			expectedParams: &EntityParams{
				ID:        1,
				IncludeHA: true,
			},
			expectedError: "",
		},
		{
			name:           "Missing ID parameter",
			url:            "/id",
			queryParams:    url.Values{},
			expectedParams: nil,
			expectedError:  "missing required parameter: id",
		},
		{
			name:           "Invalid ID format",
			url:            "/id/invalid",
			queryParams:    url.Values{},
			expectedParams: nil,
			expectedError:  "invalid ID format",
		},
		{
			name: "Invalid includeHA format",
			url:  "/id/1",
			queryParams: url.Values{
				"includeHA": []string{"invalid"},
			},
			expectedParams: &EntityParams{
				ID:        1,
				IncludeHA: true,
			},
			expectedError: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			req.URL.RawQuery = tc.queryParams.Encode()

			// Create a new router and register the route
			router := mux.NewRouter()
			router.HandleFunc("/id/{id}", func(w http.ResponseWriter, r *http.Request) {
				params, err := parseEntityParams(r)
				if tc.expectedError != "" {
					assert.Nil(t, params)
					assert.EqualError(t, err, tc.expectedError)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tc.expectedParams, params)
				}
			})

			// Serve the request
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		})
	}
}

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
