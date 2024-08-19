package mocks

import (
	"dns-api-go/internal/models"
	"errors"
)

type MockBaseService struct {
	GetEntityByIDFunc    func(id int, includeHA bool) (*models.Entity, error)
	DeleteEntityByIDFunc func(id int) error
	GetEntitiesFunc      func(start int, count int, parentId int, entityType string, includeHA bool) (*[]models.Entity, error)
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

func (m *MockBaseService) GetEntities(start int, count int, parentId int, entityType string, includeHA bool) (*[]models.Entity, error) {
	if m.GetEntitiesFunc != nil {
		return m.GetEntitiesFunc(start, count, parentId, entityType, includeHA)
	}

	return nil, errors.New("GetEntities not mocked")
}
