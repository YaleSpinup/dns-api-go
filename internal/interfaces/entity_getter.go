package interfaces

import "dns-api-go/internal/models"

type EntityGetter interface {
	GetEntity(id int, includeHA bool) (*models.Entity, error)
}
