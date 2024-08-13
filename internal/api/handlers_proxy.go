package api

import (
	"dns-api-go/logger"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	backendTimeout = 120 * time.Second
	backendPrefix  = "/v2/dns"
	contentType    = "application/json"
)

func (s *server) ProxyRequestHandler(w http.ResponseWriter, r *http.Request) {
	logger.Debug("ProxyRequestHandler invoked",
		zap.String("Method", r.Method),
		zap.String("URL", r.URL.String()),
		zap.String("Client IP", r.RemoteAddr))

	backendURL := strings.Replace(r.URL.String(), backendPrefix, s.backend.prefix, 1)
	logger.Debug("Proxying request",
		zap.String("Original URL", r.URL.String()),
		zap.String("Backend URL", backendURL))

	req, err := http.NewRequestWithContext(r.Context(), r.Method, s.backend.baseUrl+backendURL, r.Body)
	if err != nil {
		logger.Error("Failed to generate backend request", zap.String("Backend URL", backendURL), zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Auth-Token", s.backend.token)

	client := &http.Client{Timeout: backendTimeout}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("Failed to proxy request to backend", zap.Error(err))
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
