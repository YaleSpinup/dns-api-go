package api

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestBluecatAPIError_Error(t *testing.T) {
	err := &BluecatAPIError{StatusCode: 404, Body: `{"message":"not found"}`}
	got := err.Error()
	want := `bluecat api error: status 404, body: {"message":"not found"}`
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("boom"), false},
		{"404", &BluecatAPIError{StatusCode: http.StatusNotFound}, true},
		{"500", &BluecatAPIError{StatusCode: http.StatusInternalServerError}, false},
		{"401", &BluecatAPIError{StatusCode: http.StatusUnauthorized}, false},
		{"wrapped 404", fmt.Errorf("lookup: %w", &BluecatAPIError{StatusCode: http.StatusNotFound}), true},
		{"wrapped non-bluecat", fmt.Errorf("wrap: %w", errors.New("other")), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNotFound(tc.err); got != tc.want {
				t.Errorf("IsNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
