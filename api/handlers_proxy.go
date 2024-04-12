package api

import (
	"github.com/YaleSpinup/apierror"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProxyRequestHandler proxies requests to a given backend
func (s *server) ProxyRequestHandler(w http.ResponseWriter, r *http.Request) {
	log.Debugf("Method: %s", r.Method)

	url := strings.Replace(r.URL.String(), "/v2/dns", s.backend.prefix, 1)
	log.Infof("proxying request: %s to %s", r.URL, url)

	req, err := http.NewRequestWithContext(r.Context(), r.Method, s.backend.baseUrl+url, r.Body)
	if err != nil {
		log.Errorf("failed to generate backend request for %s: %s", url, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", s.backend.token)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		handleError(w, apierror.New(apierror.ErrInternalError, "failed to proxy request to backend", nil))
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
