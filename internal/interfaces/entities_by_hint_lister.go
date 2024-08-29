package interfaces

import "dns-api-go/internal/models"

type EntitiesByHintLister interface {
	GetEntitiesByHint(start int, count int, options map[string]string) (*[]models.Entity, error)
}
