package mocks

import (
	"dns-api-go/internal/models"
	"errors"
)

type MockBaseService struct {
	GetEntityFunc    func(id int, includeHA bool) (*models.Entity, error)
	DeleteEntityFunc func(id int) error
	GetEntitiesFunc  func(start int, count int, parentId int, entityType string, includeHA bool) (*[]models.Entity, error)
}

func (m *MockBaseService) GetEntity(id int, includeHA bool) (*models.Entity, error) {
	if m.GetEntityFunc != nil {
		return m.GetEntityFunc(id, includeHA)
	}

	return nil, errors.New("GetEntity not mocked")
}

func (m *MockBaseService) DeleteEntity(id int) error {
	if m.DeleteEntityFunc != nil {
		return m.DeleteEntityFunc(id)
	}

	return errors.New("DeleteEntity not mocked")
}

func (m *MockBaseService) GetEntities(start int, count int, parentId int, entityType string, includeHA bool) (*[]models.Entity, error) {
	if m.GetEntitiesFunc != nil {
		return m.GetEntitiesFunc(start, count, parentId, entityType, includeHA)
	}

	return nil, errors.New("GetEntities not mocked")
}
