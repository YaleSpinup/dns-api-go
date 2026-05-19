package common

import (
	"strings"
)

// Contains checks if a string is in a slice
func Contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

// ConvertToMap converts a string with a separator into a map of key=value
// pairs. dns-api-go's POST /ips and POST /records handlers accept a
// pipe-delimited `properties` string from server-api (its v1 wire format)
// and split it through this helper. Stays until server-api stops emitting
// pipe-delimited properties.
func ConvertToMap(inputString, separator string) map[string]string {
	if inputString == "" {
		return map[string]string{}
	}

	resultMap := make(map[string]string)
	pairs := strings.Split(inputString, separator)
	for _, pair := range pairs {
		keyValue := strings.SplitN(pair, "=", 2)
		if len(keyValue) == 2 {
			resultMap[keyValue[0]] = keyValue[1]
		}
	}
	return resultMap
}
