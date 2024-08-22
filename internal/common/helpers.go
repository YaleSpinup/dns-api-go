package common

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
