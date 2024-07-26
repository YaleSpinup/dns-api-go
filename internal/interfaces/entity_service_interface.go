package interfaces

import "dns-api-go/internal/models"

type EntityService interface {
	GetEntityByID(id int, includeHA bool) (*models.Entity, error)
}
