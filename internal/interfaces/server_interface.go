package interfaces

type ServerInterface interface {
	MakeRequest(method, route, queryParam string) ([]byte, error)
}
