package common

import (
	"dns-api-go/logger"
	"errors"
	"reflect"
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
