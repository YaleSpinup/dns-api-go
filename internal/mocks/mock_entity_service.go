package mocks

import (
	"dns-api-go/internal/models"
	"errors"
)

type MockEntityService struct {
	GetEntityByIDFunc func(id int, includeHA bool) (*models.Entity, error)
}

func (m *MockEntityService) GetEntityByID(id int, includeHA bool) (*models.Entity, error) {
	if m.GetEntityByIDFunc != nil {
		return m.GetEntityByIDFunc(id, includeHA)
	}

	return nil, errors.New("GetEntityByID not mocked")
}
