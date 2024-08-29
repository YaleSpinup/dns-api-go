package common

import (
	"dns-api-go/logger"
	"errors"
	"reflect"
	"testing"
)

// SetupLogger initializes the default logger and ensures that any buffered log entries are flushed
// before the program exits by calling logger.Sync() using defer.
func SetupLogger() {
	logger.InitializeDefault()
	defer logger.Sync()
}

// CompareErrors compares the type and message of two errors.
// It returns true if both the type and message match, otherwise false.
func CompareErrors(expectedError, actualError error) bool {
	// If either error is nil, return whether they are both nil
	if expectedError == nil || actualError == nil {
		return expectedError == actualError
	}

	// Get the type of the expected error
	expectedType := reflect.TypeOf(expectedError)
	// Create a new variable of the same type as the expected error
	targetErr := reflect.New(expectedType).Interface()

	// Check if the actual error can be cast to the type of the expected error
	if !errors.As(actualError, &targetErr) {
		return false
	}

	// Compare the error messages of both errors
	return targetErr.(error).Error() == expectedError.Error()
}

// CheckError checks the error in test cases.
func CheckError(t *testing.T, testName string, expectedError, actualError error) {
	// If json unmarshalling error, check that any error is returned
	if expectedError != nil && expectedError.Error() == "Simulating unmarshal error" {
		if actualError == nil {
			t.Errorf("%s: expected an unmarshal error, got nil", testName)
		}
		// For other errors, check if returned error matches expected error
	} else if expectedError != nil && !CompareErrors(expectedError, actualError) {
		t.Errorf("%s: expected error %v, got %v", testName, expectedError, actualError)
		// No error expected
	} else if expectedError == nil && actualError != nil {
		t.Errorf("%s: expected no error, got %v", testName, actualError)
	}
}

// CheckResponse checks the response in test cases.
func CheckResponse(t *testing.T, testName string, expectedResponse, actualResponse interface{}) {
	if !reflect.DeepEqual(expectedResponse, actualResponse) {
		t.Errorf("%s: expected response %+v, got %+v", testName, expectedResponse, actualResponse)
	}
}
