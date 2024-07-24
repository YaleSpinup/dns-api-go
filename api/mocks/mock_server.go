package mocks

type MockServer struct {
	MakeRequestFunc func(route, queryParam string) ([]byte, error)
}

func (m *MockServer) MakeRequest(route, queryParam string) ([]byte, error) {
	return m.MakeRequestFunc(route, queryParam)
}
