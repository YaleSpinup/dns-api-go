package api

import (
	"net/http"
)

// GetEntityHandler handles GET requests for retrieving an entity by ID.
func (s *server) GetEntityHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.BaseService)
}

// DeleteEntityHandler handles DELETE requests for deleting an entity by ID.
func (s *server) DeleteEntityHandler() http.HandlerFunc {
	return s.HandleDeleteEntityReq(s.services.BaseService)
}
