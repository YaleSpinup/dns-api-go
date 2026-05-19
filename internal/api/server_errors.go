package api

import "dns-api-go/internal/common"

type CIDRFileNotFound struct {
}

func (e *CIDRFileNotFound) Error() string {
	return "CIDR file not found"
}

// BluecatAPIError is re-exported from internal/common so api-package callers
// (handlers, tests) can keep using the short name without an import-cycle
// detour through services. The error type itself lives in common because
// both api and services need to construct or inspect it.
type BluecatAPIError = common.BluecatAPIError

// IsNotFound is the api-package alias for common.IsNotFound. See BluecatAPIError.
func IsNotFound(err error) bool { return common.IsNotFound(err) }
