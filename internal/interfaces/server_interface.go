package interfaces

import "io"

type ServerInterface interface {
	MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error)
	GetCIDRFile() (string, error)
	// ConfigurationID returns the cached BlueCat configuration ID. The second
	// return is false when no configurationId was supplied via config; in that
	// case callers must resolve it via the v2 API.
	ConfigurationID() (int, bool)
}
