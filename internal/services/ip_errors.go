package services

import "fmt"

type IpIncorrectActionError struct {
	Action         string
	PossibleValues []string
}

func (e *IpIncorrectActionError) Error() string {
	return fmt.Sprintf("Incorrect action: %s. Possible values: %s", e.Action, e.PossibleValues)
}
