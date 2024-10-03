package services

import "fmt"

type PoolIDError struct {
	PoolID int
	Err    error
}

func (e *PoolIDError) Error() string {
	return fmt.Sprintf("pool ID error: %s, pool ID: %d", e.Err, e.PoolID)
}
