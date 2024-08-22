package services

import (
	"fmt"
	"strings"
)

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

type ErrEntityTypeMismatch struct {
	ExpectedTypes []string
	ActualType    string
}

func (e *ErrEntityTypeMismatch) Error() string {
	return fmt.Sprintf("entity type mismatch: expected %s, got %s",
		strings.Join(e.ExpectedTypes, ", "), e.ActualType)
}
