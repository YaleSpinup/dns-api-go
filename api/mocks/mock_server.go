package mocks

import "errors"

type MockServer struct {
	MakeRequestFunc func(route, queryParam string) ([]byte, error)
}

func (m *MockServer) MakeRequest(route, queryParam string) ([]byte, error) {
	if m.MakeRequestFunc != nil {
		return m.MakeRequestFunc(route, queryParam)
	}

	return nil, errors.New("MakeRequest not mocked")
}
