package api

import (
	"errors"
	"fmt"
	"net/http"
)

type CIDRFileNotFound struct {
}

func (e *CIDRFileNotFound) Error() string {
	return "CIDR file not found"
}

// BluecatAPIError wraps a non-2xx response from the BlueCat v2 API so callers
// can distinguish 404 (entity missing) from 5xx (server error) without
// re-parsing response bodies.
type BluecatAPIError struct {
	StatusCode int
	Body       string
}

func (e *BluecatAPIError) Error() string {
	return fmt.Sprintf("bluecat api error: status %d, body: %s", e.StatusCode, e.Body)
}

// IsNotFound reports whether err is a BluecatAPIError with a 404 status.
func IsNotFound(err error) bool {
	var apiErr *BluecatAPIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}
