package api

import (
	"net/http"
)

func (s *server) GetNetworksHandler() http.HandlerFunc {
	return s.HandleGetEntitiesByHintReq(s.services.NetworkService)
}

func (s *server) GetNetworkHandler() http.HandlerFunc {
	return s.HandleGetEntityReq(s.services.NetworkService)
}
