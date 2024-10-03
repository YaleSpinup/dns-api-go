package interfaces

import "io"

type ServerInterface interface {
	MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error)
	GetCIDRFile() (string, error)
}
