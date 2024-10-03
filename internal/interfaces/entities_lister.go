package interfaces

import "dns-api-go/internal/models"

type EntitiesLister interface {
	GetEntities(start int, count int, parentId int, entityType string, includeHA bool) (*[]models.Entity, error)
}
