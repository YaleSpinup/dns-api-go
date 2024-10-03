package api

type CIDRFileNotFound struct {
}

func (e *CIDRFileNotFound) Error() string {
	return "CIDR file not found"
}
