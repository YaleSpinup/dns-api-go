package api

import (
	"dns-api-go/logger"
	"encoding/json"
	"fmt"
	"github.com/YaleSpinup/apierror"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"io"
	"net/http"
	"regexp"
)

// MakeRequest delegates to the Bluecat V2 client.
// This implements ServerInterface so services can call it unchanged.
func (s *server) MakeRequest(method, route, queryParam string, body io.Reader) ([]byte, error) {
	return s.bluecat.client.MakeRequest(method, route, queryParam, body)
}

// respond writes the response to the client
// adds a newline to the end of the response body
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

// handleError handles standard apierror return codes
func handleError(w http.ResponseWriter, err error) {
	logger.Error("API error", zap.Error(err))
	if aerr, ok := errors.Cause(err).(apierror.Error); ok {
		switch aerr.Code {
		case apierror.ErrForbidden:
			w.WriteHeader(http.StatusForbidden)
		case apierror.ErrNotFound:
			w.WriteHeader(http.StatusNotFound)
		case apierror.ErrConflict:
			w.WriteHeader(http.StatusConflict)
		case apierror.ErrBadRequest:
			w.WriteHeader(http.StatusBadRequest)
		case apierror.ErrLimitExceeded:
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(aerr.Message))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}
}

// validateMacAddress validates the format of the MAC address
// mac address should be in the format: nnnnnnnnnnnn or nn:nn:nn:nn:nn:nn or nn-nn-nn-nn-nn-nn
func validateMacAddress(macAddress string) error {
	// Define the regular expression for a valid MAC address
	macRegex := `^([0-9A-Fa-f]{12}|([0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2})$`
	re := regexp.MustCompile(macRegex)

	// Validate the MAC address format
	if !re.MatchString(macAddress) {
		return fmt.Errorf("invalid MAC address format '%s'. MAC address should be in the format: nnnnnnnnnnnn or nn:nn:nn:nn:nn:nn or nn-nn-nn-nn-nn-nn", macAddress)
	}

	return nil
}
