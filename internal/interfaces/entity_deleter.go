package interfaces

type EntityDeleter interface {
	DeleteEntity(id int) error
}
