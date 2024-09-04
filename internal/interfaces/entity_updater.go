package interfaces

import "dns-api-go/internal/models"

type EntityUpdater interface {
	UpdateEntity(entity *models.Entity) error
}
