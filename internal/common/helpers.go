package common

import (
	"fmt"
	"strings"
)

// StringPtr returns a pointer to a string
func StringPtr(s string) *string {
	return &s
}

// Contains checks if a string is in a slice
func Contains(slice []string, item string) bool {
	// Check if the item is in the slice
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// ConvertToSeparatedString converts a map to a string with a separator
func ConvertToSeparatedString(array map[string]string, separator string) string {
	// If array is empty, return empty string
	if len(array) == 0 {
		return ""
	}

	// Create key-value string pairs
	keyValuePairs := make([]string, 0, len(array))
	for key, value := range array {
		keyValuePairs = append(keyValuePairs, fmt.Sprintf("%s=%s", key, value))
	}

	return strings.Join(keyValuePairs, separator)
}

// ConvertToMap converts a string with a separator into a map
func ConvertToMap(inputString, separator string) map[string]string {
	// return empty map if propString is empty
	if inputString == "" {
		return map[string]string{}
	}

	// Initialize the map
	resultMap := make(map[string]string)

	// Split the string by the separator to get key-value pairs
	pairs := strings.Split(inputString, separator)
	for _, pair := range pairs {
		// Split each pair by "=" to get key and value
		keyValue := strings.SplitN(pair, "=", 2)
		if len(keyValue) == 2 {
			resultMap[keyValue[0]] = keyValue[1]
		}
	}

	return resultMap
}
