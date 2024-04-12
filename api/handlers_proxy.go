package api

import (
	log "github.com/sirupsen/logrus"
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
	log.Debugf("Method: %s, URL: %s, Client IP: %s", r.Method, r.URL, r.RemoteAddr)

	backendURL := strings.Replace(r.URL.String(), backendPrefix, s.backend.prefix, 1)
	log.Debugf("Proxying request: %s to %s", r.URL, backendURL)

	req, err := http.NewRequestWithContext(r.Context(), r.Method, s.backend.baseUrl+backendURL, r.Body)
	if err != nil {
		log.Errorf("Failed to generate backend request for %s: %s", backendURL, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Auth-Token", s.backend.token)

	client := &http.Client{Timeout: backendTimeout}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("Failed to proxy request to backend: %s", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
