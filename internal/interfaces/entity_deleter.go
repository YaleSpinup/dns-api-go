package interfaces

type EntityDeleter interface {
	DeleteEntity(id int, expectedTypes []string) error
}
