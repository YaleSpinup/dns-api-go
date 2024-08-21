package api

import (
	"dns-api-go/logger"
	"encoding/json"
	"go.uber.org/zap"
	"net/http"
)

func (s *server) respond(w http.ResponseWriter, data interface{}, status int) {
	w.WriteHeader(status)
	if data != nil {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(data)
		if err != nil {
			// Log failure to write the response
			logger.Error("Failed to write response", zap.Error(err))
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
		}
	}
}
