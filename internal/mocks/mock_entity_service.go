package mocks

import (
	"dns-api-go/internal/models"
	"errors"
)

type MockBaseService struct {
	GetEntityByIDFunc    func(id int, includeHA bool) (*models.Entity, error)
	DeleteEntityByIDFunc func(id int) error
}

func (m *MockBaseService) GetEntityByID(id int, includeHA bool) (*models.Entity, error) {
	if m.GetEntityByIDFunc != nil {
		return m.GetEntityByIDFunc(id, includeHA)
	}

	return nil, errors.New("GetEntityByID not mocked")
}

func (m *MockBaseService) DeleteEntityByID(id int) error {
	if m.DeleteEntityByIDFunc != nil {
		return m.DeleteEntityByIDFunc(id)
	}

	return errors.New("DeleteEntityByID not mocked")
}
