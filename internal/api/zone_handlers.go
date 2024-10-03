package api

import (
	"net/http"
)

func (s *server) GetZonesHandler() http.HandlerFunc {
	return s.HandleGetEntitiesByHintReq(s.services.ZoneService)
}

func (s *server) GetZoneHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.ZoneService)
}
