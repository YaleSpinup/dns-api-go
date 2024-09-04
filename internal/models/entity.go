package models

type Entity struct {
	ID         int
	Name       string
	Type       string
	Properties map[string]string
}
