package services

import "fmt"

// ErrDeleteNotAllowed indicates an operation is not allowed
type ErrDeleteNotAllowed struct {
	Type string
}

func (e *ErrDeleteNotAllowed) Error() string {
	return fmt.Sprintf("deletion not allowed for entity type %s", e.Type)
}

// ErrEntityNotFound indicates the entity was not found
type ErrEntityNotFound struct{}

func (e *ErrEntityNotFound) Error() string {
	return "entity not found"
}
