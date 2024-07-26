package interfaces

type ServerInterface interface {
	MakeRequest(route, queryParam string) ([]byte, error)
}
