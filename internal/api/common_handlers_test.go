package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
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

			router := mux.NewRouter()
			router.HandleFunc("/id/{id}", func(w http.ResponseWriter, r *http.Request) {
				params, err := parseEntityParams(r)
				if tc.expectedError != "" {
					assert.EqualError(t, err, tc.expectedError)
					assert.Nil(t, params)
					return
				}
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedParams, params)
			})

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		})
	}
}
