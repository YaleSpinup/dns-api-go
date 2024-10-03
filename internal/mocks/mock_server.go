package mocks

import (
	"errors"
	"io"
)

type MockServer struct {
	MakeRequestFunc func(method, route, queryParam string, body io.Reader) ([]byte, error)
	GetCIDRFileFunc func() (string, error)
}

func (m *MockServer) MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error) {
	if m.MakeRequestFunc != nil {
		return m.MakeRequestFunc(method, route, queryParam, body)
	}

	return nil, errors.New("MakeRequest not mocked")
}

func (m *MockServer) GetCIDRFile() (string, error) {
	if m.GetCIDRFileFunc != nil {
		return m.GetCIDRFileFunc()
	}

	return "", errors.New("GetCIDRFile not mocked")
}
