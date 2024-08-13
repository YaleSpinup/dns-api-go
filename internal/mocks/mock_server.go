package mocks

import "errors"

type MockServer struct {
	MakeRequestFunc func(method, route, queryParam string) ([]byte, error)
}

func (m *MockServer) MakeRequest(method, route, queryParam string) ([]byte, error) {
	if m.MakeRequestFunc != nil {
		return m.MakeRequestFunc(method, route, queryParam)
	}

	return nil, errors.New("MakeRequest not mocked")
}
